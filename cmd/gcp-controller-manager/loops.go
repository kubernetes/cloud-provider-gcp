/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"sort"

	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

type controllerContext struct {
	client                             clientset.Interface
	sharedInformers                    informers.SharedInformerFactory
	recorder                           record.EventRecorder
	gcpCfg                             gcpConfig
	clusterSigningGKEKubeconfig        string
	csrApproverVerifyClusterMembership bool
	csrApproverAllowLegacyKubelet      bool
	verifiedSAs                        *saMap
	done                               <-chan struct{}
	hmsAuthorizeSAMappingURL           string
	hmsSyncNodeURL                     string
	delayDirectPathGSARemove           bool
}

// loops returns all the control loops that the GCPControllerManager can start.
// We append GCP to all of these to disambiguate them in API server and audit
// logs. These loops are intentionally started in a random order.
func loops() map[string]func(*controllerContext) error {
	ll := map[string]func(*controllerContext) error{
		"certificate-approver": func(ctx *controllerContext) error {
			approver := newGKEApprover(ctx)
			approveController := certificates.NewCertificateController(
				"approver",
				ctx.client,
				ctx.sharedInformers.Certificates().V1().CertificateSigningRequests(),
				approver.handle,
			)
			go approveController.Run(20, ctx.done)
			return nil
		},
		"certificate-signer": func(ctx *controllerContext) error {
			signer, err := newGKESigner(ctx)
			if err != nil {
				return err
			}
			signController := certificates.NewCertificateController(
				"signer",
				ctx.client,
				ctx.sharedInformers.Certificates().V1().CertificateSigningRequests(),
				signer.handle,
			)

			go signController.Run(20, ctx.done)
			return nil
		},
		"node-annotator": func(ctx *controllerContext) error {
			nodeAnnotateController, err := newNodeAnnotator(
				ctx.client,
				ctx.sharedInformers.Core().V1().Nodes(),
				ctx.gcpCfg.Compute,
			)
			if err != nil {
				return err
			}
			go nodeAnnotateController.Run(5, ctx.done)
			return nil
		},
	}
	if *directPath {
		ll[saVerifierControlLoopName] = func(ctx *controllerContext) error {
			serviceAccountVerifier, err := newServiceAccountVerifier(
				ctx.client,
				ctx.sharedInformers.Core().V1().ServiceAccounts(),
				ctx.sharedInformers.Core().V1().ConfigMaps(),
				ctx.gcpCfg.Compute,
				ctx.verifiedSAs,
				ctx.hmsAuthorizeSAMappingURL,
			)
			if err != nil {
				return err
			}
			go serviceAccountVerifier.Run(3, ctx.done)
			return nil
		}
		ll[nodeSyncerControlLoopName] = func(ctx *controllerContext) error {
			nodeSyncer, err := newNodeSyncer(
				ctx.sharedInformers.Core().V1().Pods(),
				ctx.verifiedSAs,
				ctx.hmsSyncNodeURL,
				ctx.client,
				ctx.delayDirectPathGSARemove,
			)
			if err != nil {
				return err
			}
			go nodeSyncer.Run(10, ctx.done)
			return nil
		}
	}
	return ll
}

func loopNames() []string {
	names := make([]string, 0)
	for name := range loops() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
