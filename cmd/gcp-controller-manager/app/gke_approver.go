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

package app

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"sort"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/golang/glog"
	"github.com/google/go-tpm/tpm2"
	"golang.org/x/oauth2"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	gcfg "gopkg.in/gcfg.v1"
	warnings "gopkg.in/warnings.v0"
	authorization "k8s.io/api/authorization/v1beta1"
	capi "k8s.io/api/certificates/v1beta1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
	"k8s.io/cloud-provider-gcp/pkg/tpmattest"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1beta1"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

const (
	legacyKubeletUsername = "kubelet"
	tpmKubeletUsername    = "kubelet-bootstrap"

	// TODO(awly): eventually this should be true. But that makes testing
	// harder, so disabling or now.
	validateClusterMembership = false
)

// gkeApprover handles approval/denial of CSRs based on SubjectAccessReview and
// CSR attestation data.
type gkeApprover struct {
	client     clientset.Interface
	opts       approverOptions
	validators []csrValidator
}

func newGKEApprover(opts approverOptions, client clientset.Interface) *gkeApprover {
	return &gkeApprover{client: client, validators: validators, opts: opts}
}

func (a *gkeApprover) handle(csr *capi.CertificateSigningRequest) error {
	if len(csr.Status.Certificate) != 0 {
		return nil
	}
	if approved, denied := certificates.GetCertApprovalCondition(&csr.Status); approved || denied {
		return nil
	}
	glog.Infof("approver got CSR %q", csr.Name)

	x509cr, err := certutil.ParseCSR(csr)
	if err != nil {
		return fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}

	var tried []string
	for _, r := range a.validators {
		if !r.recognize(a.opts, csr, x509cr) {
			continue
		}
		glog.Infof("validator %q: matched CSR %q", r.name, csr.Name)
		tried = append(tried, r.name)
		if r.validate != nil && !r.validate(a.opts, csr, x509cr) {
			glog.Infof("validator %q: denied CSR %q", r.name, csr.Name)
			return a.updateCSR(csr, false, r.denyMsg)
		}
		glog.Infof("CSR %q validation passed", csr.Name)

		approved, err := a.authorizeSAR(csr, r.permission)
		if err != nil {
			return err
		}
		if approved {
			glog.Infof("validator %q: SubjectAccessReview approved for CSR %q", r.name, csr.Name)
			return a.updateCSR(csr, true, r.approveMsg)
		} else {
			glog.Warningf("validator %q: SubjectAccessReview denied for CSR %q", r.name, csr.Name)
		}
	}

	if len(tried) != 0 {
		return certificates.IgnorableError("recognized csr %q as %q but subject access review was not approved", csr.Name, tried)
	}
	glog.Infof("no validators matched CSR %q", csr.Name)
	return nil
}

func (a *gkeApprover) updateCSR(csr *capi.CertificateSigningRequest, approved bool, msg string) error {
	if approved {
		csr.Status.Conditions = append(csr.Status.Conditions, capi.CertificateSigningRequestCondition{
			Type:    capi.CertificateApproved,
			Reason:  "AutoApproved",
			Message: msg,
		})
	} else {
		csr.Status.Conditions = append(csr.Status.Conditions, capi.CertificateSigningRequestCondition{
			Type:    capi.CertificateDenied,
			Reason:  "AutoDenied",
			Message: msg,
		})
	}
	_, err := a.client.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(csr)
	if err != nil {
		return fmt.Errorf("error updating approval status for csr: %v", err)
	}
	return nil
}

type csrCheckFunc func(opts approverOptions, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool

type csrValidator struct {
	name       string
	approveMsg string
	denyMsg    string

	// recognize is a required field that returns true if this csrValidator is
	// applicable to given CSR.
	recognize csrCheckFunc
	// validate is an optional field that returns true whether CSR should be
	// approved or denied.
	// If validate returns false, CSR is denied immediately.
	// If validate returns true, CSR proceeds to SubjectAccessReview check.
	validate csrCheckFunc

	permission authorization.ResourceAttributes
}

// More specific validators go first.
var validators = []csrValidator{
	{
		name:       "kubelet client certificate with TPM attestation and SubjectAccessReview",
		recognize:  isNodeClientCertWithAttestation,
		validate:   validateTPMAttestation,
		permission: authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "nodeclient"},
		approveMsg: "Auto approving kubelet client certificate with TPM attestation after SubjectAccessReview.",
	},
	{
		name:       "self kubelet client certificate SubjectAccessReview",
		recognize:  isSelfNodeClientCert,
		permission: authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "selfnodeclient"},
		approveMsg: "Auto approving self kubelet client certificate after SubjectAccessReview.",
	},
	{
		name:       "kubelet client certificate SubjectAccessReview",
		recognize:  isLegacyNodeClientCert,
		permission: authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "nodeclient"},
		approveMsg: "Auto approving kubelet client certificate after SubjectAccessReview.",
	},
	{
		name:       "kubelet server certificate SubjectAccessReview",
		recognize:  isNodeServerCert,
		validate:   validateNodeServerCert,
		permission: authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "selfnodeclient"},
		approveMsg: "Auto approving kubelet server certificate after SubjectAccessReview.",
	},
}

func (a *gkeApprover) authorizeSAR(csr *capi.CertificateSigningRequest, rattrs authorization.ResourceAttributes) (bool, error) {
	extra := make(map[string]authorization.ExtraValue)
	for k, v := range csr.Spec.Extra {
		extra[k] = authorization.ExtraValue(v)
	}

	sar := &authorization.SubjectAccessReview{
		Spec: authorization.SubjectAccessReviewSpec{
			User:               csr.Spec.Username,
			UID:                csr.Spec.UID,
			Groups:             csr.Spec.Groups,
			Extra:              extra,
			ResourceAttributes: &rattrs,
		},
	}
	sar, err := a.client.AuthorizationV1beta1().SubjectAccessReviews().Create(sar)
	if err != nil {
		return false, err
	}
	return sar.Status.Allowed, nil
}

func hasExactUsages(csr *capi.CertificateSigningRequest, usages []capi.KeyUsage) bool {
	if len(usages) != len(csr.Spec.Usages) {
		return false
	}

	usageMap := map[capi.KeyUsage]struct{}{}
	for _, u := range usages {
		usageMap[u] = struct{}{}
	}

	for _, u := range csr.Spec.Usages {
		if _, ok := usageMap[u]; !ok {
			return false
		}
	}

	return true
}

var (
	kubeletClientUsages = []capi.KeyUsage{
		capi.UsageKeyEncipherment,
		capi.UsageDigitalSignature,
		capi.UsageClientAuth,
	}
	kubeletServerUsages = []capi.KeyUsage{
		capi.UsageKeyEncipherment,
		capi.UsageDigitalSignature,
		capi.UsageServerAuth,
	}
)

func isNodeCert(opts approverOptions, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !reflect.DeepEqual([]string{"system:nodes"}, x509cr.Subject.Organization) {
		return false
	}
	if len(x509cr.EmailAddresses) > 0 {
		return false
	}
	return strings.HasPrefix(x509cr.Subject.CommonName, "system:node:")
}

func isNodeClientCert(opts approverOptions, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeCert(opts, csr, x509cr) {
		return false
	}
	if len(x509cr.DNSNames) > 0 || len(x509cr.IPAddresses) > 0 {
		return false
	}
	return hasExactUsages(csr, kubeletClientUsages)
}

func isLegacyNodeClientCert(opts approverOptions, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeClientCert(opts, csr, x509cr) {
		return false
	}
	return csr.Spec.Username == legacyKubeletUsername
}

func isSelfNodeClientCert(opts approverOptions, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeClientCert(opts, csr, x509cr) {
		return false
	}
	return csr.Spec.Username == x509cr.Subject.CommonName
}

func isNodeServerCert(opts approverOptions, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeCert(opts, csr, x509cr) {
		return false
	}
	if !hasExactUsages(csr, kubeletServerUsages) {
		return false
	}
	return csr.Spec.Username == x509cr.Subject.CommonName
}

func validateNodeServerCert(opts approverOptions, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	client := oauth2.NewClient(context.Background(), opts.tokenSource)

	if err := validateNodeServerCertInner(client, opts, csr, x509cr); err != nil {
		glog.Errorf("validating CSR %q: %v", csr.Name, err)
		return false
	}
	return true
}

// Only check that IPs in SAN match an existing VM in the project.
// Username was already checked against CN, so this CSR is coming from
// authenticated kubelet.
func validateNodeServerCertInner(client *http.Client, opts approverOptions, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) error {
	switch {
	case len(x509cr.IPAddresses) == 0:
		return errors.New("no SAN IPs")
	case len(x509cr.EmailAddresses) > 0 || len(x509cr.URIs) > 0:
		return errors.New("only DNS and IP SANs allowed")
	}

	cs, err := compute.New(client)
	if err != nil {
		return fmt.Errorf("creating GCE API client: %v", err)
	}

	srv := compute.NewInstancesService(cs)
	instanceName := strings.TrimPrefix(csr.Spec.Username, "system:node:")
	for _, z := range opts.zones {
		inst, err := srv.Get(opts.projectID, z, instanceName).Do()
		if err != nil {
			continue
		}
	scanIPs:
		for _, ip := range x509cr.IPAddresses {
			for _, iface := range inst.NetworkInterfaces {
				if ip.String() == iface.NetworkIP {
					continue scanIPs
				}
				for _, ac := range iface.AccessConfigs {
					if ip.String() == ac.NatIP {
						continue scanIPs
					}
				}
			}
			return fmt.Errorf("IP addresses in CSR (%q) don't match NetworkInterfaces on instance %q (%+v)", x509cr.IPAddresses, instanceName, inst.NetworkInterfaces)
		}
		return nil
	}
	return fmt.Errorf("Instance name %q doesn't match any VM in cluster project/zone: %v", instanceName, err)
}

type approverOptions struct {
	clusterName string
	projectID   string
	zones       []string
	tokenSource oauth2.TokenSource
	tpmCACache  *caCache
}

func loadApproverOptions(s *GCPControllerManager) (approverOptions, error) {
	var a approverOptions

	// Load gce.conf.
	gceConfig := struct {
		Global struct {
			ProjectID string `gcfg:"project-id"`
			TokenURL  string `gcfg:"token-url"`
			TokenBody string `gcfg:"token-body"`
		}
	}{}
	// ReadFileInfo will return warnings for extra fields in gce.conf we don't
	// care about. Wrap with FatalOnly to discard those.
	if err := warnings.FatalOnly(gcfg.ReadFileInto(&gceConfig, s.GCEConfigPath)); err != nil {
		return a, err
	}
	a.projectID = gceConfig.Global.ProjectID
	// Get the token source for GCE API.
	a.tokenSource = gce.NewAltTokenSource(gceConfig.Global.TokenURL, gceConfig.Global.TokenBody)

	// Get cluster zone from metadata server.
	zone, err := metadata.Zone()
	if err != nil {
		return a, err
	}
	// Extract region name from zone.
	if len(zone) < 2 {
		return a, fmt.Errorf("invalid master zone: %q", zone)
	}
	region := zone[:len(zone)-2]
	// Load all zones in the same region.
	client := oauth2.NewClient(context.Background(), a.tokenSource)
	cs, err := compute.New(client)
	if err != nil {
		return a, fmt.Errorf("creating GCE API client: %v", err)
	}
	allZones, err := compute.NewZonesService(cs).List(a.projectID).Do()
	if err != nil {
		return a, err
	}
	for _, z := range allZones.Items {
		if strings.HasPrefix(z.Name, region) {
			a.zones = append(a.zones, z.Name)
		}
	}
	if len(a.zones) == 0 {
		return a, fmt.Errorf("can't find zones for region %q", region)
	}
	// Put master's zone first.
	sort.Slice(a.zones, func(i, j int) bool { return a.zones[i] == zone })

	a.clusterName, err = metadata.Get("instance/attributes/cluster-name")
	if err != nil {
		return a, err
	}

	a.tpmCACache = &caCache{
		rootCertURL: rootCertURL,
		interPrefix: intermediateCAPrefix,
		certs:       make(map[string]*x509.Certificate),
		crls:        make(map[string]*cachedCRL),
	}

	return a, nil
}

var tpmAttestationBlocks = []string{
	"CERTIFICATE REQUEST",
	"ATTESTATION CERTIFICATE",
	"ATTESTATION DATA",
	"ATTESTATION SIGNATURE",
	"VM IDENTITY",
}

func isNodeClientCertWithAttestation(opts approverOptions, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeClientCert(opts, csr, x509cr) {
		return false
	}
	if csr.Spec.Username != tpmKubeletUsername {
		return false
	}
	blocks, err := parsePEMBlocks(csr.Spec.Request)
	if err != nil {
		glog.Errorf("parsing csr.Spec.Request: %v", err)
		return false
	}
	for _, name := range tpmAttestationBlocks {
		if _, ok := blocks[name]; !ok {
			return false
		}
	}
	return true
}

func validateTPMAttestation(opts approverOptions, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	blocks, err := parsePEMBlocks(csr.Spec.Request)
	if err != nil {
		glog.Errorf("Parsing csr.Spec.Request: %v", err)
		return false
	}
	nodeIDRaw := blocks["VM IDENTITY"].Bytes
	attestDataRaw := blocks["ATTESTATION DATA"].Bytes
	attestSig := blocks["ATTESTATION SIGNATURE"].Bytes
	attestCert := blocks["ATTESTATION CERTIFICATE"].Bytes

	var nodeID nodeidentity.Identity
	if err := json.Unmarshal(nodeIDRaw, &nodeID); err != nil {
		glog.Errorf("Unmarshaling VM identity JSON: %v", err)
		return false
	}
	if fmt.Sprint(nodeID.ProjectName) != opts.projectID {
		glog.Errorf("Received CSR for a different project Name (%d)", nodeID.ProjectName)
		return false
	}

	client := oauth2.NewClient(context.Background(), opts.tokenSource)
	cs, err := compute.New(client)
	if err != nil {
		glog.Errorf("Creating GCE API client: %v", err)
		return false
	}
	srv := compute.NewInstancesService(cs)
	inst, err := srv.Get(fmt.Sprint(nodeID.ProjectID), nodeID.Zone, nodeID.Name).Do()
	if err != nil {
		glog.Errorf("Fetching VM data from GCE API: %v", err)
		return false
	}
	if validateClusterMembership {
		ok, err := clusterHasInstance(client, opts.projectID, inst.Zone, opts.clusterName, inst.Id)
		if err != nil {
			glog.Errorf("Checking VM membership in cluster: %v", err)
			return false
		}
		if !ok {
			glog.Errorf("VM %q doesn't belong to cluster %q", inst.Name, opts.clusterName)
			return false
		}
	}

	// TODO(awly): get AIK public key from GCE API.
	aikCert, err := x509.ParseCertificate(attestCert)
	if err != nil {
		glog.Errorf("Parsing ATTESTATION_CERTIFICATE: %v", err)
		return false
	}
	if err := opts.tpmCACache.verify(aikCert); err != nil {
		glog.Errorf("Verifying EK certificate validity: %v", err)
		return false
	}
	aikPub, ok := aikCert.PublicKey.(*rsa.PublicKey)
	if !ok {
		glog.Errorf("Public key in ATTESTATION CERTIFICATE is %T, want *rsa.PublicKey", aikCert.PublicKey)
		return false
	}

	attestHash := sha256.Sum256(attestDataRaw)
	if err := rsa.VerifyPKCS1v15(aikPub, crypto.SHA256, attestHash[:], attestSig); err != nil {
		glog.Errorf("Verifying certification signature with AIK public key: %v", err)
		return false
	}

	// Verify that attestDataRaw matches certificate.
	attestData, err := tpm2.DecodeAttestationData(attestDataRaw)
	if err != nil {
		glog.Errorf("Parsing attestation data in CSR: %v", err)
		return false
	}
	pub, err := tpmattest.MakePublic(x509cr.PublicKey)
	if err != nil {
		glog.Errorf("Converting public key in CSR to TPM Public structure: %v", err)
		return false
	}
	ok, err = attestData.AttestedCertifyInfo.Name.MatchesPublic(pub)
	if err != nil {
		glog.Errorf("Comparing ATTESTATION DATA to CSR public key: %v", err)
		return false
	}
	if !ok {
		glog.Errorf("ATTESTATION DATA doesn't match CSR public key")
		return false
	}
	return true
}

func parsePEMBlocks(raw []byte) (map[string]*pem.Block, error) {
	blocks := make(map[string]*pem.Block)
	for {
		// Just in case there are extra newlines between blocks.
		raw = bytes.TrimSpace(raw)

		var b *pem.Block
		b, raw = pem.Decode(raw)
		if b == nil {
			break
		}
		blocks[b.Type] = b
	}
	if len(blocks) == 0 {
		return nil, errors.New("no valid PEM blocks found in CSR")
	}
	if len(raw) != 0 {
		return nil, errors.New("trailing non-PEM data in CSR")
	}
	return blocks, nil
}

func clusterHasInstance(client *http.Client, project, zone, clusterName string, instanceID uint64) (bool, error) {
	cs, err := container.New(client)
	if err != nil {
		return false, err
	}
	cluster, err := container.NewProjectsZonesClustersService(cs).Get(project, zone, clusterName).Do()
	if err != nil {
		return false, err
	}
	for _, np := range cluster.NodePools {
		for _, ig := range np.InstanceGroupUrls {
			igName := path.Base(ig)
			ok, err := groupHasInstance(client, project, zone, igName, instanceID)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
	}
	return false, nil
}

func groupHasInstance(client *http.Client, project, zone, groupName string, instanceID uint64) (bool, error) {
	cs, err := compute.New(client)
	if err != nil {
		return false, err
	}
	instances, err := compute.NewInstanceGroupManagersService(cs).ListManagedInstances(project, zone, groupName).Do()
	if err != nil {
		return false, err
	}
	for _, inst := range instances.ManagedInstances {
		if instanceID == inst.Id {
			return true, nil
		}
	}
	return false, nil
}
