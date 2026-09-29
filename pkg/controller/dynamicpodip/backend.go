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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip/ane"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

const (
	// defaultANEMutationConcurrency is the maximum number of concurrent ANE creation
	// or deletion operations executed in parallel per node.
	defaultANEMutationConcurrency = 8
)

// PodIPBackend abstracts the cloud IP allocation operations (either GCE Alias IP ranges or ANEs).
type PodIPBackend interface {
	// GetNetworkInterfaces returns the network interfaces with their allocated Pod CIDRs.
	GetNetworkInterfaces(ctx context.Context, providerID string) ([]*networkInterface, error)
	// Mutate applies additions and removals of Pod IPs on the cloud provider.
	Mutate(ctx context.Context, providerID, networkURL string, additions []string, removals []string, candidateRanges []string) error
	// CalculateAdditions determines the CIDR allocations needed to satisfy the demand for neededIPs.
	CalculateAdditions(network string, neededIPs int, currentIfaces []*networkInterface) []string
	// IsANEMode returns true if ANE mode is enabled.
	IsANEMode() bool
}

// aliasRangesBackend implements PodIPBackend using GCE VM instance alias IP ranges.
type aliasRangesBackend struct {
	gceCloud *gce.Cloud
}

// NewAliasRangesBackend constructs a PodIPBackend using standard GCE Alias IP ranges.
func NewAliasRangesBackend(gceCloud *gce.Cloud) PodIPBackend {
	return &aliasRangesBackend{gceCloud: gceCloud}
}

func (b *aliasRangesBackend) GetNetworkInterfaces(ctx context.Context, providerID string) ([]*networkInterface, error) {
	gceIfaces, err := b.gceCloud.GetInstanceNetworkInterfaces(ctx, providerID)
	if err != nil {
		return nil, err
	}
	return toNetworkInterfaces(gceIfaces), nil
}

func (b *aliasRangesBackend) Mutate(ctx context.Context, providerID, networkURL string, additions []string, removals []string, candidateRanges []string) error {
	return b.gceCloud.UpdateInstanceAliasIPRanges(ctx, providerID, networkURL, additions, removals, candidateRanges)
}

func (b *aliasRangesBackend) CalculateAdditions(network string, neededIPs int, currentIfaces []*networkInterface) []string {
	blocksNeeded := (neededIPs + DefaultCapacity - 1) / DefaultCapacity
	var additions []string
	for i := 0; i < blocksNeeded; i++ {
		additions = append(additions, DefaultBlockSize)
	}
	return additions
}

func (b *aliasRangesBackend) IsANEMode() bool {
	return false
}

// aneBackend implements PodIPBackend using GCE AliasNetworkEndpoints.
type aneBackend struct {
	gceCloud  *gce.Cloud
	aneClient ane.Client
}

// NewANEBackend constructs a PodIPBackend using GCE AliasNetworkEndpoints.
func NewANEBackend(gceCloud *gce.Cloud, aneClient ane.Client) PodIPBackend {
	return &aneBackend{
		gceCloud:  gceCloud,
		aneClient: aneClient,
	}
}

func (b *aneBackend) IsANEMode() bool {
	return true
}

func (b *aneBackend) CalculateAdditions(network string, neededIPs int, currentIfaces []*networkInterface) []string {
	var additions []string
	for i := 0; i < neededIPs; i++ {
		additions = append(additions, "/32")
	}
	return additions
}

func (b *aneBackend) GetNetworkInterfaces(ctx context.Context, providerID string) ([]*networkInterface, error) {
	project, zone, name, err := gce.SplitProviderID(providerID)
	if err != nil {
		return nil, err
	}
	name = gce.CanonicalizeInstanceName(name)
	instanceURL := fmt.Sprintf("projects/%s/zones/%s/instances/%s", project, zone, name)

	anes, err := b.aneClient.ListByInstance(ctx, project, zone, instanceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list ANEs for instance %q: %w", name, err)
	}

	var ifaces []*networkInterface
	ifaceSubnets := make(map[string]string)
	if b.gceCloud != nil {
		if gceIfaces, err := b.gceCloud.GetInstanceNetworkInterfaces(ctx, providerID); err == nil && len(gceIfaces) > 0 {
			ifaces = toNetworkInterfaces(gceIfaces)
			for _, gi := range gceIfaces {
				if gi != nil {
					ifaceSubnets[gi.Name] = gi.Subnetwork
				}
			}
		}
	}
	if len(ifaces) == 0 {
		primaryNetworkURL := ""
		if b.gceCloud != nil {
			primaryNetworkURL = b.gceCloud.NetworkURL()
		}
		ifaces = []*networkInterface{
			{
				Name:    "nic0",
				Network: primaryNetworkURL,
			},
		}
	}

	for _, endpoint := range anes {
		if endpoint == nil || endpoint.Status == nil {
			continue
		}
		if endpoint.Status.State != ane.StateActive {
			// Skip endpoints that are in transient states (CREATING, DELETING)
			continue
		}
		alias, ok := endpoint.Aliases[ane.DefaultAliasName]
		if !ok || alias == nil {
			return nil, fmt.Errorf("active ANE %q on instance %q has no alias %q (potential silent scrubbing or API version mismatch)", endpoint.Name, name, ane.DefaultAliasName)
		}
		ip := alias.EffectiveIPAddress
		if ip == "" {
			ip = alias.IPAddress
		}
		if ip == "" {
			return nil, fmt.Errorf("active ANE %q on instance %q has empty IP address (potential silent scrubbing or API version mismatch)", endpoint.Name, name)
		}

		cidr := ip
		if !strings.Contains(cidr, "/") {
			cidr += "/32"
		}
		targetIface := ifaces[0]
		if endpoint.Subnetwork != "" {
			for _, iface := range ifaces {
				if subnet, ok := ifaceSubnets[iface.Name]; ok && gce.EqualResourceURLs(subnet, endpoint.Subnetwork) {
					targetIface = iface
					break
				}
			}
		}
		targetIface.AliasIPRanges = append(targetIface.AliasIPRanges, cidr)
	}

	return ifaces, nil
}

func (b *aneBackend) Mutate(ctx context.Context, providerID, networkURL string, additions []string, removals []string, candidateRanges []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	project, zone, name, err := gce.SplitProviderID(providerID)
	if err != nil {
		return err
	}
	name = gce.CanonicalizeInstanceName(name)
	instanceURL := fmt.Sprintf("projects/%s/zones/%s/instances/%s", project, zone, name)

	// Removals: find matching ANEs by IP and delete them concurrently
	if len(removals) > 0 {
		anes, err := b.aneClient.ListByInstance(ctx, project, zone, instanceURL)
		if err != nil {
			return fmt.Errorf("failed to list ANEs for removal on instance %q: %w", name, err)
		}

		ipToANEName := make(map[string]string)
		for _, ep := range anes {
			if ep != nil {
				if alias, ok := ep.Aliases[ane.DefaultAliasName]; ok && alias != nil {
					ip := alias.EffectiveIPAddress
					if ip == "" {
						ip = alias.IPAddress
					}
					if ip != "" {
						ipToANEName[ip] = ep.Name
						ipToANEName[ip+"/32"] = ep.Name
					}
				}
			}
		}

		toDelete := sets.NewString()
		for _, rem := range removals {
			aneName, ok := ipToANEName[rem]
			if !ok {
				trimmed := strings.TrimSuffix(rem, "/32")
				aneName, ok = ipToANEName[trimmed]
			}
			if !ok {
				klog.Warningf("ANE for removal CIDR %q not found on node %q, skipping", rem, name)
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
					klog.Infof("Deleting ANE %q from node %q", aneName, name)
					opName, err := b.aneClient.Delete(gCtx, project, zone, aneName)
					if err != nil {
						return fmt.Errorf("failed to delete ANE %q: %w", aneName, err)
					}
					if opName != "" {
						if err := b.aneClient.WaitForOperation(gCtx, project, zone, opName); err != nil {
							return fmt.Errorf("failed waiting for ANE %q deletion: %w", aneName, err)
						}
					}
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				return err
			}
		}
	}

	// Additions: create new ANEs for the instance concurrently
	if len(additions) > 0 {
		subnetURL := ""
		if b.gceCloud != nil {
			subnetURL = b.gceCloud.SubnetworkURL()
			if gceIfaces, err := b.gceCloud.GetInstanceNetworkInterfaces(ctx, providerID); err == nil {
				for _, iface := range gceIfaces {
					if iface != nil && (networkURL == "" || gce.EqualResourceURLs(iface.Network, networkURL)) {
						if iface.Subnetwork != "" {
							subnetURL = iface.Subnetwork
							break
						}
					}
				}
			}
		}
		var candidateRanges []string
		if b.gceCloud != nil {
			if secRange := b.gceCloud.SecondaryRangeName(); secRange != "" {
				candidateRanges = []string{secRange}
			}
		}

		g, gCtx := errgroup.WithContext(ctx)
		g.SetLimit(defaultANEMutationConcurrency)

		for i := range additions {
			idx := i
			aneName := generateANEName(name, idx)
			endpoint := &ane.AliasNetworkEndpoint{
				Name:                   aneName,
				Subnetwork:             subnetURL,
				Host:                   &ane.Host{Instance: instanceURL},
				SecurityTagInheritance: ane.SecurityTagInheritanceInherited,
				Aliases: map[string]*ane.Alias{
					ane.DefaultAliasName: {
						IPVersion:            ane.DefaultIPVersion,
						SubnetworkRangeNames: candidateRanges,
					},
				},
			}

			g.Go(func() error {
				if err := gCtx.Err(); err != nil {
					return err
				}
				klog.Infof("Creating ANE %q on node %q (subnetwork %q)", aneName, name, subnetURL)
				opName, err := b.aneClient.Insert(gCtx, project, zone, endpoint)
				if err != nil {
					return fmt.Errorf("failed to insert ANE %q: %w", aneName, err)
				}
				if opName != "" {
					if err := b.aneClient.WaitForOperation(gCtx, project, zone, opName); err != nil {
						return fmt.Errorf("failed waiting for ANE %q creation: %w", aneName, err)
					}
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}
	}

	return nil
}

// generateANEName creates an RFC 1035 compliant resource name for an ANE that is guaranteed
// to be at most 63 characters long:
// ane-<node-prefix>-<timestamp-hash>-<index>
func generateANEName(nodeName string, index int) string {
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
	name := fmt.Sprintf("ane-%s%s", cleanNode, suffix)
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}
