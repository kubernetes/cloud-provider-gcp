/*
Copyright 2026 The Kubernetes Authors.

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

package dynamicpodip

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	computepreview "google.golang.org/api/compute/v1.preview"
	"k8s.io/apimachinery/pkg/util/sets"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

const (
	// DefaultANEAliasNameIPv4 is the client-chosen RFC 1035 map key used in
	// AliasNetworkEndpoint.Aliases when creating an IPv4 Pod IP endpoint.
	// The GCE API requires a user-provided RFC 1035 identifier as the map
	// key and currently limits each endpoint to at most one entry
	// (max_length: 1).
	//
	// WARNING: Do not change this value. Modifying this constant will
	// prevent the controller from recognizing existing ANE aliases across
	// upgrades, causing NodeNetworkConfig drift and Pod IP disruption.
	DefaultANEAliasNameIPv4 = "gke-pod-ipv4"

	// DefaultANEDescription is the description set on AliasNetworkEndpoint
	// resources created by the dynamic Pod IP controller.
	DefaultANEDescription = "Managed by cloud-controller-manager for GKE Adaptive Cluster IPAM"

	// defaultANEMutationConcurrency is the maximum number of concurrent ANE
	// creation or deletion operations executed in parallel per node.
	//
	// Each ANE is one GCE operation of ~5.5s (p50) to ~12s (p90), and a
	// single scale-up on a busy node routinely asks for 20-40 of them. With
	// a limit of 8 those were serialized into 3-5 waves, so a pod waiting
	// on the last wave saw 30-40s. 50 lets a typical burst complete in one
	// wave; the effective ceiling is then the project's ANE operation
	// quota rather than this constant.
	defaultANEMutationConcurrency = 50

	// aneTopUpPollInterval bounds how often an AdditionDemandFunc is
	// consulted while creations are in flight and no additions are waiting
	// to be launched. The demand function reads from an informer cache, so
	// this is cheap; it mainly limits log noise.
	aneTopUpPollInterval = 500 * time.Millisecond

	// maxANETopUpPerMutate caps how many additions a single Mutate call may
	// absorb from its demand function on top of the initial batch. It is a
	// safety bound against a daemon that keeps raising its ask faster than
	// endpoints can be created; anything beyond it is left to the next
	// reconcile, which is exactly the pre-top-up behaviour.
	maxANETopUpPerMutate = 256
)

// aneBackend implements PodIPBackend using GCE AliasNetworkEndpoints.
type aneBackend struct {
	gceCloud *gce.Cloud
}

// NewANEBackend constructs a PodIPBackend using GCE AliasNetworkEndpoints.
func NewANEBackend(gceCloud *gce.Cloud) PodIPBackend {
	return &aneBackend{gceCloud: gceCloud}
}

func (b *aneBackend) CalculateAdditions(network string, neededIPs int, currentIfaces []*networkInterface) []string {
	if neededIPs <= 0 {
		return nil
	}
	additions := make([]string, neededIPs)
	for i := range additions {
		additions[i] = "/32"
	}
	return additions
}

func (b *aneBackend) GetNetworkInterfaces(ctx context.Context, providerID string) ([]*networkInterface, error) {
	gceIfaces, err := b.gceCloud.GetInstanceNetworkInterfaces(ctx, providerID)
	if err != nil {
		return nil, err
	}
	ifaces := toNetworkInterfaces(gceIfaces)
	if len(ifaces) == 0 {
		return nil, fmt.Errorf("instance %q has no network interfaces", providerID)
	}

	anes, err := b.gceCloud.ListAliasNetworkEndpoints(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list ANEs for %q: %w", providerID, err)
	}

	for _, endpoint := range anes {
		if endpoint == nil || endpoint.Status == nil {
			continue
		}
		if endpoint.Status.State != gce.ANEStateActive {
			// Skip endpoints in transient states (CREATING,
			// DELETING) and mark the interface so the spec and
			// status controllers requeue until they settle.
			klog.Warningf("Skipping non-active ANE %q on %q (state=%q); will retry reconciliation", endpoint.Name, providerID, endpoint.Status.State)
			ifaces[0].HasTransientANE = true
			continue
		}
		alias, ok := endpoint.Aliases[DefaultANEAliasNameIPv4]
		if !ok {
			return nil, fmt.Errorf("active ANE %q on %q has no alias %q (potential silent scrubbing or API version mismatch)", endpoint.Name, providerID, DefaultANEAliasNameIPv4)
		}
		if alias.EffectiveIpAddress == "" {
			return nil, fmt.Errorf("active ANE %q on %q has empty IP address (potential silent scrubbing or API version mismatch)", endpoint.Name, providerID)
		}

		cidr := alias.EffectiveIpAddress + "/32"
		targetIface := ifaces[0]
		if endpoint.Subnetwork != "" {
			var matched *networkInterface
			for _, iface := range ifaces {
				if iface != nil && gce.EqualResourceURLs(iface.Subnetwork, endpoint.Subnetwork) {
					matched = iface
					break
				}
			}
			if matched == nil {
				klog.Warningf("Skipping ANE %q on %q: subnetwork %q does not match any network interface on instance", endpoint.Name, providerID, endpoint.Subnetwork)
				continue
			}
			targetIface = matched
		}
		targetIface.ANECIDRs = append(targetIface.ANECIDRs, cidr)
		if targetIface.ANENames == nil {
			targetIface.ANENames = make(map[string]string)
		}
		targetIface.ANENames[cidr] = endpoint.Name
	}

	return ifaces, nil
}

func (b *aneBackend) Mutate(ctx context.Context, providerID, networkURL string, additions []string, removals []string, candidateRanges []string) error {
	return b.MutateWithTopUp(ctx, providerID, networkURL, additions, removals, candidateRanges, nil)
}

// MutateWithTopUp implements TopUpBackend.
func (b *aneBackend) MutateWithTopUp(ctx context.Context, providerID, networkURL string, additions []string, removals []string, candidateRanges []string, more AdditionDemandFunc) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Removals: find matching ANEs by CIDR and delete them concurrently.
	if len(removals) > 0 {
		anes, err := b.gceCloud.ListAliasNetworkEndpoints(ctx, providerID)
		if err != nil {
			return fmt.Errorf("failed to list ANEs for removal on %q: %w", providerID, err)
		}

		cidrToANEName := make(map[string]string, len(anes))
		for _, ep := range anes {
			if ep == nil {
				continue
			}
			alias, ok := ep.Aliases[DefaultANEAliasNameIPv4]
			if !ok || alias.EffectiveIpAddress == "" {
				continue
			}
			cidrToANEName[alias.EffectiveIpAddress+"/32"] = ep.Name
		}

		toDelete := sets.NewString()
		for _, rem := range removals {
			aneName, ok := cidrToANEName[rem]
			if !ok {
				klog.Warningf("ANE for removal CIDR %q not found on %q, skipping", rem, providerID)
				continue
			}
			toDelete.Insert(aneName)
		}

		if toDelete.Len() > 0 {
			g, gCtx := errgroup.WithContext(ctx)
			g.SetLimit(defaultANEMutationConcurrency)
			for _, targetName := range toDelete.List() {
				aneName := targetName
				g.Go(func() error {
					if err := gCtx.Err(); err != nil {
						return err
					}
					klog.Infof("Deleting ANE %q from %q", aneName, providerID)
					if err := b.gceCloud.DeleteAliasNetworkEndpoint(gCtx, providerID, aneName); err != nil {
						return fmt.Errorf("failed to delete ANE %q: %w", aneName, err)
					}
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				return err
			}
		}
	}

	// Additions: create new ANEs for the instance concurrently. Every
	// addition is a /32, so the slice length is the number of endpoints.
	if len(additions) > 0 {
		subnetURL, err := b.resolveSubnetworkURL(ctx, providerID, networkURL)
		if err != nil {
			return err
		}
		return b.createANEs(ctx, providerID, subnetURL, candidateRanges, len(additions), more)
	}

	return nil
}

// newANEEndpoint builds the AliasNetworkEndpoint request body for the
// index-th endpoint created for providerID in this mutation.
func (b *aneBackend) newANEEndpoint(providerID, subnetURL string, candidateRanges []string, index int) *computepreview.AliasNetworkEndpoint {
	return &computepreview.AliasNetworkEndpoint{
		Name:                   generateANEName(providerID, index),
		Description:            DefaultANEDescription,
		Subnetwork:             subnetURL,
		SecurityTagInheritance: gce.ANESecurityTagInheritanceInherited,
		Aliases: map[string]computepreview.AliasNetworkEndpointAlias{
			DefaultANEAliasNameIPv4: {
				IpVersion:            gce.ANEIPVersionIPv4,
				SubnetworkRangeNames: candidateRanges,
			},
		},
	}
}

// createANEs creates initial endpoints for providerID with at most
// defaultANEMutationConcurrency operations in flight, and, if more is
// non-nil, keeps the slots busy with any further demand it reports.
//
// The dispatcher is a single goroutine that owns all the counters; workers
// only report completion on a channel. It consults more when launching
// would otherwise stall -- i.e. there is nothing left to launch but
// operations are still in flight -- and once more after the last operation
// completes, so demand that arrived during the final operation is served
// here rather than by a follow-up reconcile.
//
// On the first failure the shared context is cancelled, which aborts the
// remaining in-flight operations the same way the errgroup did; the
// dispatcher then waits for them to report before returning the error.
func (b *aneBackend) createANEs(ctx context.Context, providerID, subnetURL string, candidateRanges []string, initial int, more AdditionDemandFunc) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan error, defaultANEMutationConcurrency)
	var (
		pending  = initial // decided, not yet launched
		inFlight int
		launched int // also the next endpoint index
		toppedUp int
		firstErr error
	)

	launch := func() {
		endpoint := b.newANEEndpoint(providerID, subnetURL, candidateRanges, launched)
		launched++
		pending--
		inFlight++
		go func() {
			if err := ctx.Err(); err != nil {
				results <- err
				return
			}
			klog.Infof("Creating ANE %q on %q (subnetwork %q)", endpoint.Name, providerID, subnetURL)
			if err := b.gceCloud.CreateAliasNetworkEndpoint(ctx, providerID, endpoint); err != nil {
				results <- fmt.Errorf("failed to create ANE %q: %w", endpoint.Name, err)
				return
			}
			results <- nil
		}()
	}

	poll := func() {
		if more == nil || firstErr != nil || ctx.Err() != nil || toppedUp >= maxANETopUpPerMutate {
			return
		}
		// Do not start work that cannot finish within the reconcile budget.
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < gce.DefaultANEOperationTimeout {
			return
		}
		extra, err := more(ctx)
		if err != nil {
			klog.Warningf("Failed to check for additional ANE demand on %q, leaving it to the next reconcile: %v", providerID, err)
			return
		}
		n := len(extra)
		if n > maxANETopUpPerMutate-toppedUp {
			n = maxANETopUpPerMutate - toppedUp
		}
		if n <= 0 {
			return
		}
		klog.Infof("Absorbing %d additional ANE(s) for %q into the in-flight batch (initial=%d, topped up so far=%d)", n, providerID, initial, toppedUp)
		pending += n
		toppedUp += n
	}

	ticker := time.NewTicker(aneTopUpPollInterval)
	defer ticker.Stop()

	for {
		for firstErr == nil && pending > 0 && inFlight < defaultANEMutationConcurrency {
			launch()
		}
		if inFlight == 0 {
			if firstErr != nil {
				return firstErr
			}
			// Nothing running and nothing queued: one last look for demand
			// that arrived during the final operation.
			poll()
			if pending == 0 {
				return nil
			}
			continue
		}
		select {
		case err := <-results:
			inFlight--
			if err != nil && firstErr == nil {
				firstErr = err
				cancel()
			}
		case <-ticker.C:
		}
		// Only look for more work when the slots would otherwise go idle.
		if pending == 0 {
			poll()
		}
	}
}

func (b *aneBackend) resolveSubnetworkURL(ctx context.Context, providerID, networkURL string) (string, error) {
	gceIfaces, err := b.gceCloud.GetInstanceNetworkInterfaces(ctx, providerID)
	if err != nil {
		return "", err
	}
	for _, iface := range gceIfaces {
		if iface != nil && (networkURL == "" || gce.EqualResourceURLs(iface.Network, networkURL)) {
			if iface.Subnetwork != "" {
				return iface.Subnetwork, nil
			}
		}
	}
	return "", fmt.Errorf("no subnetwork found on instance %q for network %q", providerID, networkURL)
}

// generateANEName creates an RFC 1035 compliant resource name for an ANE that
// is guaranteed to be at most 63 characters long:
// ane-<node-prefix>-<timestamp-hash>-<index>
//
// Note: The generated name is only used at creation time to ensure zonal name
// uniqueness. Lookups and deletions always match ANEs by Host.Instance and
// EffectiveIpAddress rather than parsing or reconstructing the resource name,
// so future changes to this format will not affect existing endpoints.
func generateANEName(nodeOrProviderID string, index int) string {
	nodeName := lastComponent(nodeOrProviderID)
	if dot := strings.Index(nodeName, "."); dot >= 0 {
		nodeName = nodeName[:dot]
	}

	var sb strings.Builder
	for _, ch := range strings.ToLower(nodeName) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			sb.WriteRune(ch)
		} else {
			sb.WriteRune('-')
		}
	}
	cleanNode := strings.Trim(sb.String(), "-")

	suffix := fmt.Sprintf("-%x-%d", time.Now().UnixNano()%0xFFFFFF, index)
	maxNodeLen := 63 - len("ane-") - len(suffix)
	if len(cleanNode) > maxNodeLen {
		cleanNode = cleanNode[:maxNodeLen]
	}
	cleanNode = strings.Trim(cleanNode, "-")
	if cleanNode == "" {
		cleanNode = "node"
	}
	return fmt.Sprintf("ane-%s%s", cleanNode, suffix)
}
