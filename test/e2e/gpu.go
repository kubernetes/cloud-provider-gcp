/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"

	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/kubernetes/test/e2e/chaosmonkey"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/utils/junit"
	admissionapi "k8s.io/pod-security-admission/api"
)

// UpgradeType represents different types of upgrades.
type UpgradeType int

const (
	// MasterUpgrade indicates that only the master is being upgraded.
	MasterUpgrade UpgradeType = iota

	// NodeUpgrade indicates that only the nodes are being upgraded.
	NodeUpgrade

	// ClusterUpgrade indicates that both master and nodes are
	// being upgraded.
	ClusterUpgrade

	// EtcdUpgrade indicates that only etcd is being upgraded (or migrated
	// between storage versions).
	EtcdUpgrade
)

// Test is an interface for upgrade tests.
type Test interface {
	// Name should return a test name sans spaces.
	Name() string

	// Setup should create and verify whatever objects need to
	// exist before the upgrade disruption starts.
	Setup(ctx context.Context, f *framework.Framework)

	// Test will run during the upgrade. When the upgrade is
	// complete, done will be closed and final validation can
	// begin.
	Test(ctx context.Context, f *framework.Framework, done <-chan struct{}, upgrade UpgradeType)

	// Teardown should clean up any objects that are created that
	// aren't already cleaned up by the framework. This will
	// always be called, even if Setup failed.
	Teardown(ctx context.Context, f *framework.Framework)
}

// Skippable is an interface that an upgrade test can implement to be
// able to indicate that it should be skipped.
type Skippable interface {
	// Skip should return true if test should be skipped. upgCtx
	// provides information about the upgrade that is going to
	// occur.
	Skip(upgCtx UpgradeContext) bool
}

// UpgradeContext contains information about all the stages of the
// upgrade that is going to occur.
type UpgradeContext struct {
	Versions []VersionContext
}

// VersionContext represents a stage of the upgrade.
type VersionContext struct {
	Version   version.Version
	NodeImage string
}

var upgradeTests = []Test{
	&NvidiaGPUUpgradeTest{},
}

var _ = ginkgo.Describe("[cloud-provider-gcp-e2e] GPU Upgrade", func() {
	f := framework.NewDefaultFramework("gpu-upgrade")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged
	testFrameworks := CreateUpgradeFrameworks(upgradeTests)

	ginkgo.Describe("master upgrade", func() {
		f.It("should NOT disrupt gpu pod", func(ctx context.Context) {
			upgCtx, err := GetUpgradeContext(f.ClientSet.Discovery())
			framework.ExpectNoError(err)

			testSuite := &junit.TestSuite{Name: "GPU master upgrade"}
			gpuUpgradeTest := &junit.TestCase{Name: "gpu-master-upgrade", Classname: "upgrade_tests"}
			testSuite.TestCases = append(testSuite.TestCases, gpuUpgradeTest)

			upgradeFunc := ControlPlaneUpgradeFunc(f, upgCtx, gpuUpgradeTest, nil)
			RunUpgradeSuite(ctx, upgCtx, upgradeTests, testFrameworks, testSuite, MasterUpgrade, upgradeFunc)
		})
	})
	ginkgo.Describe("cluster upgrade", func() {
		f.It("should be able to run gpu pod after upgrade", func(ctx context.Context) {
			upgCtx, err := GetUpgradeContext(f.ClientSet.Discovery())
			framework.ExpectNoError(err)

			testSuite := &junit.TestSuite{Name: "GPU cluster upgrade"}
			gpuUpgradeTest := &junit.TestCase{Name: "gpu-cluster-upgrade", Classname: "upgrade_tests"}
			testSuite.TestCases = append(testSuite.TestCases, gpuUpgradeTest)

			upgradeFunc := ClusterUpgradeFunc(f, upgCtx, gpuUpgradeTest, nil, nil)
			RunUpgradeSuite(ctx, upgCtx, upgradeTests, testFrameworks, testSuite, ClusterUpgrade, upgradeFunc)
		})
	})
	ginkgo.Describe("cluster downgrade", func() {
		f.It("should be able to run gpu pod after downgrade", func(ctx context.Context) {
			upgCtx, err := GetUpgradeContext(f.ClientSet.Discovery())
			framework.ExpectNoError(err)

			testSuite := &junit.TestSuite{Name: "GPU cluster downgrade"}
			gpuDowngradeTest := &junit.TestCase{Name: "gpu-cluster-downgrade", Classname: "upgrade_tests"}
			testSuite.TestCases = append(testSuite.TestCases, gpuDowngradeTest)

			upgradeFunc := ClusterDowngradeFunc(f, upgCtx, gpuDowngradeTest, nil, nil)
			RunUpgradeSuite(ctx, upgCtx, upgradeTests, testFrameworks, testSuite, ClusterUpgrade, upgradeFunc)
		})
	})
})

func CreateUpgradeFrameworks(tests []Test) map[string]*framework.Framework {
	nsFilter := regexp.MustCompile("[^[:word:]-]+") // match anything that's not a word character or hyphen
	testFrameworks := map[string]*framework.Framework{}
	for _, t := range tests {
		ns := nsFilter.ReplaceAllString(t.Name(), "-") // and replace with a single hyphen
		ns = strings.Trim(ns, "-")
		f := framework.NewDefaultFramework(ns)
		f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged
		testFrameworks[t.Name()] = f
	}
	return testFrameworks
}

type chaosMonkeyAdapter struct {
	test        Test
	framework   *framework.Framework
	upgradeType UpgradeType
	upgCtx      UpgradeContext
}

func (cma *chaosMonkeyAdapter) Test(ctx context.Context, sem *chaosmonkey.Semaphore) {
	var once sync.Once
	ready := func() {
		once.Do(func() {
			sem.Ready()
		})
	}
	defer ready()
	if skippable, ok := cma.test.(Skippable); ok && skippable.Skip(cma.upgCtx) {
		ginkgo.By("skipping test " + cma.test.Name())
		return
	}

	ginkgo.DeferCleanup(cma.test.Teardown, cma.framework)
	cma.test.Setup(ctx, cma.framework)
	ready()
	cma.test.Test(ctx, cma.framework, sem.StopCh, cma.upgradeType)
}

func RunUpgradeSuite(
	ctx context.Context,
	upgCtx *UpgradeContext,
	tests []Test,
	testFrameworks map[string]*framework.Framework,
	testSuite *junit.TestSuite,
	upgradeType UpgradeType,
	upgradeFunc func(ctx context.Context),
) {
	cm := chaosmonkey.New(upgradeFunc)
	for _, t := range tests {
		testCase := &junit.TestCase{
			Name:      t.Name(),
			Classname: "upgrade_tests",
		}
		testSuite.TestCases = append(testSuite.TestCases, testCase)
		cma := chaosMonkeyAdapter{
			test:        t,
			framework:   testFrameworks[t.Name()],
			upgradeType: upgradeType,
			upgCtx:      *upgCtx,
		}
		cm.Register(cma.Test)
	}

	start := time.Now()
	defer func() {
		testSuite.Update()
		testSuite.Time = time.Since(start).Seconds()
		if framework.TestContext.ReportDir != "" {
			fname := filepath.Join(framework.TestContext.ReportDir, fmt.Sprintf("junit_%supgrades.xml", framework.TestContext.ReportPrefix))
			f, err := os.Create(fname)
			if err != nil {
				return
			}
			defer f.Close()
			xml.NewEncoder(f).Encode(testSuite)
		}
	}()
	cm.Do(ctx)
}
