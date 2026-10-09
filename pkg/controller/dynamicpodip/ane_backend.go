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
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

const (
	// defaultANEMutationConcurrency is the maximum number of concurrent ANE
	// creation or deletion operations executed in parallel per node.
	defaultANEMutationConcurrency = 8
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
	var additions []string
	for i := 0; i < neededIPs; i++ {
		additions = append(additions, "/32")
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
			// DELETING).
			continue
		}
		alias, ok := endpoint.Aliases[gce.DefaultANEAliasNameIPv4]
		if !ok || alias == nil {
			return nil, fmt.Errorf("active ANE %q on %q has no alias %q (potential silent scrubbing or API version mismatch)", endpoint.Name, providerID, gce.DefaultANEAliasNameIPv4)
		}
		if alias.EffectiveIPAddress == "" {
			return nil, fmt.Errorf("active ANE %q on %q has empty IP address (potential silent scrubbing or API version mismatch)", endpoint.Name, providerID)
		}

		cidr := alias.EffectiveIPAddress
		if !strings.Contains(cidr, "/") {
			cidr += "/32"
		}
		targetIface := ifaces[0]
		if endpoint.Subnetwork != "" {
			for _, iface := range ifaces {
				if iface != nil && gce.EqualResourceURLs(iface.Subnetwork, endpoint.Subnetwork) {
					targetIface = iface
					break
				}
			}
		}
		targetIface.ANECIDRs = append(targetIface.ANECIDRs, cidr)
	}

	return ifaces, nil
}

func (b *aneBackend) Mutate(ctx context.Context, providerID, networkURL string, additions []string, removals []string, candidateRanges []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Removals: find matching ANEs by IP and delete them concurrently.
	if len(removals) > 0 {
		anes, err := b.gceCloud.ListAliasNetworkEndpoints(ctx, providerID)
		if err != nil {
			return fmt.Errorf("failed to list ANEs for removal on %q: %w", providerID, err)
		}

		ipToANEName := make(map[string]string)
		for _, ep := range anes {
			if ep != nil {
				if alias, ok := ep.Aliases[gce.DefaultANEAliasNameIPv4]; ok && alias != nil && alias.EffectiveIPAddress != "" {
					ipToANEName[alias.EffectiveIPAddress] = ep.Name
					ipToANEName[alias.EffectiveIPAddress+"/32"] = ep.Name
				}
			}
		}

		toDelete := sets.NewString()
		for _, rem := range removals {
			aneName, ok := ipToANEName[rem]
			if !ok {
				aneName, ok = ipToANEName[strings.TrimSuffix(rem, "/32")]
			}
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

	// Additions: create new ANEs for the instance concurrently.
	if len(additions) > 0 {
		subnetURL := b.resolveSubnetworkURL(ctx, providerID, networkURL)
		if len(candidateRanges) == 0 {
			if secRange := b.gceCloud.SecondaryRangeName(); secRange != "" {
				candidateRanges = []string{secRange}
			}
		}

		g, gCtx := errgroup.WithContext(ctx)
		g.SetLimit(defaultANEMutationConcurrency)

		for i := range additions {
			aneName := generateANEName(providerID, i)
			endpoint := &gce.AliasNetworkEndpoint{
				Name:                   aneName,
				Description:            gce.DefaultANEDescription,
				Subnetwork:             subnetURL,
				SecurityTagInheritance: gce.ANESecurityTagInheritanceInherited,
				Aliases: map[string]*gce.ANEAlias{
					gce.DefaultANEAliasNameIPv4: {
						IPVersion:            gce.DefaultANEIPVersion,
						SubnetworkRangeNames: candidateRanges,
					},
				},
			}

			g.Go(func() error {
				if err := gCtx.Err(); err != nil {
					return err
				}
				klog.Infof("Creating ANE %q on %q (subnetwork %q)", aneName, providerID, subnetURL)
				if err := b.gceCloud.CreateAliasNetworkEndpoint(gCtx, providerID, endpoint); err != nil {
					return fmt.Errorf("failed to create ANE %q: %w", aneName, err)
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

func (b *aneBackend) resolveSubnetworkURL(ctx context.Context, providerID, networkURL string) string {
	subnetURL := b.gceCloud.SubnetworkURL()
	if gceIfaces, err := b.gceCloud.GetInstanceNetworkInterfaces(ctx, providerID); err == nil {
		for _, iface := range gceIfaces {
			if iface != nil && (networkURL == "" || gce.EqualResourceURLs(iface.Network, networkURL)) {
				if iface.Subnetwork != "" {
					return iface.Subnetwork
				}
			}
		}
	}
	return subnetURL
}

// generateANEName creates an RFC 1035 compliant resource name for an ANE that
// is guaranteed to be at most 63 characters long:
// ane-<node-prefix>-<timestamp-hash>-<index>
//
// Note: The generated name is only used at creation time to ensure zonal name
// uniqueness. Lookups and deletions always match ANEs by Host.Instance and
// EffectiveIPAddress rather than parsing or reconstructing the resource name,
// so future changes to this format will not affect existing endpoints.
func generateANEName(nodeOrProviderID string, index int) string {
	nodeName := nodeOrProviderID
	if slash := strings.LastIndex(nodeName, "/"); slash >= 0 {
		nodeName = nodeName[slash+1:]
	}
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
	name := fmt.Sprintf("ane-%s%s", cleanNode, suffix)
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}
