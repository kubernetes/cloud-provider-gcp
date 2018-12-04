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
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/google/go-tpm/tpm2"
	"github.com/prometheus/client_golang/prometheus"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
	authorization "k8s.io/api/authorization/v1beta1"
	capi "k8s.io/api/certificates/v1beta1"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
	"k8s.io/cloud-provider-gcp/pkg/tpmattest"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1beta1"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

var (
	csrApprovalStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "csr_approval_count",
		Help: "Count of approved, denied and ignored CSRs",
	}, []string{"status", "kind"})
	csrApprovalLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "csr_approval_latencies",
		Help: "Latency of CSR approver, in seconds",
	}, []string{"status", "kind"})
)

func init() {
	prometheus.MustRegister(csrApprovalStatus)
	prometheus.MustRegister(csrApprovalLatency)
}

const (
	legacyKubeletUsername = "kubelet"
	tpmKubeletUsername    = "kubelet-bootstrap"

	authFlowLabelNone = "not_approved"
)

// gkeApprover handles approval/denial of CSRs based on SubjectAccessReview and
// CSR attestation data.
type gkeApprover struct {
	client     clientset.Interface
	opts       GCPConfig
	validators []csrValidator
}

func newGKEApprover(opts GCPConfig, client clientset.Interface) *gkeApprover {
	return &gkeApprover{client: client, validators: validators, opts: opts}
}

func (a *gkeApprover) handle(csr *capi.CertificateSigningRequest) error {
	start := time.Now()
	if len(csr.Status.Certificate) != 0 {
		return nil
	}
	if approved, denied := certificates.GetCertApprovalCondition(&csr.Status); approved || denied {
		return nil
	}
	glog.Infof("approver got CSR %q", csr.Name)

	x509cr, err := certutil.ParseCSR(csr)
	if err != nil {
		csrApprovalStatus.WithLabelValues("parse_error", authFlowLabelNone).Inc()
		csrApprovalLatency.WithLabelValues("parse_error", authFlowLabelNone).Observe(time.Since(start).Seconds())
		return fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}

	var tried []string
	for _, r := range a.validators {
		if !r.recognize(a.opts, csr, x509cr) {
			continue
		}
		glog.Infof("validator %q: matched CSR %q", r.name, csr.Name)
		tried = append(tried, r.name)
		if r.validate != nil {
			ok, err := r.validate(a.opts, csr, x509cr)
			if err != nil {
				return fmt.Errorf("validating CSR %q: %v", csr.Name, err)
			}
			if !ok {
				glog.Infof("validator %q: denied CSR %q", r.name, csr.Name)
				csrApprovalStatus.WithLabelValues("deny", r.authFlowLabel).Inc()
				csrApprovalLatency.WithLabelValues("deny", r.authFlowLabel).Observe(time.Since(start).Seconds())
				return a.updateCSR(csr, false, r.denyMsg)
			}
		}
		glog.Infof("CSR %q validation passed", csr.Name)

		approved, err := a.authorizeSAR(csr, r.permission)
		if err != nil {
			csrApprovalStatus.WithLabelValues("sar_error", r.authFlowLabel).Inc()
			csrApprovalLatency.WithLabelValues("sar_error", r.authFlowLabel).Observe(time.Since(start).Seconds())
			return err
		}
		if !approved {
			glog.Warningf("validator %q: SubjectAccessReview denied for CSR %q", r.name, csr.Name)
			continue
		}
		glog.Infof("validator %q: SubjectAccessReview approved for CSR %q", r.name, csr.Name)
		if r.preApproveHook != nil {
			if err := r.preApproveHook(a.opts, csr, x509cr, r.authFlowLabel, a.client); err != nil {
				glog.Warningf("validator %q: preApproveHook failed for CSR %q: %v", r.name, csr.Name, err)
				csrApprovalStatus.WithLabelValues("pre_approve_hook_error", r.authFlowLabel).Inc()
				csrApprovalLatency.WithLabelValues("pre_approve_hook_error", r.authFlowLabel).Observe(time.Since(start).Seconds())
				return err
			}
			glog.Infof("validator %q: preApproveHook passed for CSR %q", r.name, csr.Name)
		}
		csrApprovalStatus.WithLabelValues("approve", r.authFlowLabel).Inc()
		csrApprovalLatency.WithLabelValues("approve", r.authFlowLabel).Observe(time.Since(start).Seconds())
		return a.updateCSR(csr, true, r.approveMsg)
	}

	if len(tried) != 0 {
		csrApprovalStatus.WithLabelValues("sar_reject", authFlowLabelNone).Inc()
		csrApprovalLatency.WithLabelValues("sar_reject", authFlowLabelNone).Observe(time.Since(start).Seconds())
		return certificates.IgnorableError("recognized csr %q as %q but subject access review was not approved", csr.Name, tried)
	}
	glog.Infof("no validators matched CSR %q", csr.Name)
	csrApprovalStatus.WithLabelValues("ignore", authFlowLabelNone).Inc()
	csrApprovalLatency.WithLabelValues("ignore", authFlowLabelNone).Observe(time.Since(start).Seconds())
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

type recognizeFunc func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool
type validateFunc func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error)
type preApproveHookFunc func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest, metricsLabel string, client clientset.Interface) error

type csrValidator struct {
	name          string
	authFlowLabel string
	approveMsg    string
	denyMsg       string

	// recognize is a required field that returns true if this csrValidator is
	// applicable to given CSR.
	recognize recognizeFunc
	// validate is an optional field that returns true whether CSR should be
	// approved or denied.
	// If validate returns an error, CSR will be retried.
	// If validate returns (false, nil), CSR is denied immediately.
	// If validate returns (true, nil), CSR proceeds to SubjectAccessReview
	// check.
	//
	// validate should only return errors for temporary problems.
	validate validateFunc

	permission authorization.ResourceAttributes

	// preApproveHook is an optional function that runs immediately before a CSR is approved (after recognize/validate/permission checks have passed).
	// If preApproveHook returns an error, the CSR will be retried.
	// If preApproveHook returns no error, the CSR will be approved.
	preApproveHook preApproveHookFunc
}

// More specific validators go first.
var validators = []csrValidator{
	{
		name:          "kubelet client certificate with TPM attestation and SubjectAccessReview",
		authFlowLabel: "kubelet_client_tpm",
		recognize:     isNodeClientCertWithAttestation,
		validate:      validateTPMAttestation,
		permission:    authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "nodeclient"},
		approveMsg:    "Auto approving kubelet client certificate with TPM attestation after SubjectAccessReview.",

		preApproveHook: ensureNodeMatchesMetadataOrDelete,
	},
	{
		name:          "self kubelet client certificate SubjectAccessReview",
		authFlowLabel: "kubelet_client_self",
		recognize:     isSelfNodeClientCert,
		permission:    authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "selfnodeclient"},
		approveMsg:    "Auto approving self kubelet client certificate after SubjectAccessReview.",
	},
	{
		name:          "kubelet client certificate SubjectAccessReview",
		authFlowLabel: "kubelet_client_legacy",
		recognize:     isLegacyNodeClientCert,
		permission:    authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "nodeclient"},
		approveMsg:    "Auto approving kubelet client certificate after SubjectAccessReview.",

		preApproveHook: ensureNodeMatchesMetadataOrDelete,
	},
	{
		name:          "kubelet server certificate SubjectAccessReview",
		authFlowLabel: "kubelet_server_self",
		recognize:     isNodeServerCert,
		validate:      validateNodeServerCert,
		permission:    authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "selfnodeclient"},
		approveMsg:    "Auto approving kubelet server certificate after SubjectAccessReview.",
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

func isNodeCert(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !reflect.DeepEqual([]string{"system:nodes"}, x509cr.Subject.Organization) {
		return false
	}
	if len(x509cr.EmailAddresses) > 0 {
		return false
	}
	return strings.HasPrefix(x509cr.Subject.CommonName, "system:node:")
}

func isNodeClientCert(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeCert(opts, csr, x509cr) {
		return false
	}
	if len(x509cr.DNSNames) > 0 || len(x509cr.IPAddresses) > 0 {
		return false
	}
	return hasExactUsages(csr, kubeletClientUsages)
}

func isLegacyNodeClientCert(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeClientCert(opts, csr, x509cr) {
		return false
	}
	return csr.Spec.Username == legacyKubeletUsername
}

func isSelfNodeClientCert(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeClientCert(opts, csr, x509cr) {
		return false
	}
	return csr.Spec.Username == x509cr.Subject.CommonName
}

func isNodeServerCert(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeCert(opts, csr, x509cr) {
		return false
	}
	if !hasExactUsages(csr, kubeletServerUsages) {
		return false
	}
	return csr.Spec.Username == x509cr.Subject.CommonName
}

// Only check that IPs in SAN match an existing VM in the project.
// Username was already checked against CN, so this CSR is coming from
// authenticated kubelet.
func validateNodeServerCert(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
	switch {
	case len(x509cr.IPAddresses) == 0:
		glog.Infof("deny CSR %q: no SAN IPs", csr.Name)
		return false, nil
	case len(x509cr.EmailAddresses) > 0 || len(x509cr.URIs) > 0:
		glog.Infof("deny CSR %q: only DNS and IP SANs allowed", csr.Name)
		return false, nil
	}

	srv := compute.NewInstancesService(opts.Compute)
	instanceName := strings.TrimPrefix(csr.Spec.Username, "system:node:")
	for _, z := range opts.Zones {
		inst, err := srv.Get(opts.ProjectID, z, instanceName).Do()
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return false, err
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
			glog.Infof("deny CSR %q: IP addresses in CSR (%q) don't match NetworkInterfaces on instance %q (%+v)", csr.Name, x509cr.IPAddresses, instanceName, inst.NetworkInterfaces)
			return false, nil
		}
		return true, nil
	}
	glog.Infof("deny CSR %q: instance name %q doesn't match any VM in cluster project/zone", csr.Name, instanceName)
	return false, nil
}

var tpmAttestationBlocks = []string{
	"CERTIFICATE REQUEST",
	"ATTESTATION CERTIFICATE",
	"ATTESTATION DATA",
	"ATTESTATION SIGNATURE",
}

func isNodeClientCertWithAttestation(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
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

func validateTPMAttestation(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
	blocks, err := parsePEMBlocks(csr.Spec.Request)
	if err != nil {
		glog.Infof("deny CSR %q: parsing csr.Spec.Request: %v", csr.Name, err)
		return false, nil
	}
	attestDataRaw := blocks["ATTESTATION DATA"].Bytes
	attestSig := blocks["ATTESTATION SIGNATURE"].Bytes
	attestCert := blocks["ATTESTATION CERTIFICATE"].Bytes

	// TODO(awly): get AIK public key from GCE API.
	aikCert, err := x509.ParseCertificate(attestCert)
	if err != nil {
		glog.Infof("deny CSR %q: parsing ATTESTATION_CERTIFICATE: %v", csr.Name, err)
		return false, nil
	}
	if err := opts.TPMEndorsementCACache.verify(aikCert); err != nil {
		glog.Infof("deny CSR %q: verifying EK certificate validity: %v", csr.Name, err)
		return false, nil
	}
	aikPub, ok := aikCert.PublicKey.(*rsa.PublicKey)
	if !ok {
		glog.Infof("deny CSR %q: public key in ATTESTATION CERTIFICATE is %T, want *rsa.PublicKey", csr.Name, aikCert.PublicKey)
		return false, nil
	}

	nodeID, err := nodeidentity.FromAIKCert(aikCert)
	if err != nil {
		glog.Infof("deny CSR %q: failed extracting VM identity from EK certificate: %v", csr.Name, err)
		return false, nil
	}
	hostname := strings.TrimPrefix(x509cr.Subject.CommonName, "system:node:")
	if nodeID.Name != hostname {
		glog.Infof("deny CSR %q: VM name in ATTESTATION CERTIFICATE (%q) doesn't match CommonName in x509 CSR (%q)", csr.Name, nodeID.Name, x509cr.Subject.CommonName)
		return false, nil
	}
	if fmt.Sprint(nodeID.ProjectName) != opts.ProjectID {
		glog.Infof("deny CSR %q: received CSR for a different project Name (%d)", csr.Name, nodeID.ProjectName)
		return false, nil
	}

	srv := compute.NewInstancesService(opts.Compute)
	inst, err := srv.Get(fmt.Sprint(nodeID.ProjectID), nodeID.Zone, nodeID.Name).Do()
	if err != nil {
		if isNotFound(err) {
			glog.Infof("deny CSR %q: VM doesn't exist in GCE API: %v", csr.Name, err)
			return false, nil
		}
		return false, fmt.Errorf("fetching VM data from GCE API: %v", err)
	}
	if opts.VerifyClusterMembership {
		ok, err = clusterHasInstance(opts, inst.Zone, inst.Id)
		if err != nil {
			return false, fmt.Errorf("checking VM membership in cluster: %v", err)
		}
		if !ok {
			glog.Infof("deny CSR %q: VM %q doesn't belong to cluster %q", csr.Name, inst.Name, opts.ClusterName)
			return false, nil
		}
	}

	attestHash := sha256.Sum256(attestDataRaw)
	if err := rsa.VerifyPKCS1v15(aikPub, crypto.SHA256, attestHash[:], attestSig); err != nil {
		glog.Infof("deny CSR %q: verifying certification signature with AIK public key: %v", csr.Name, err)
		return false, nil
	}

	// Verify that attestDataRaw matches certificate.
	attestData, err := tpm2.DecodeAttestationData(attestDataRaw)
	if err != nil {
		glog.Infof("deny CSR %q: parsing attestation data in CSR: %v", csr.Name, err)
		return false, nil
	}
	pub, err := tpmattest.MakePublic(x509cr.PublicKey)
	if err != nil {
		glog.Infof("deny CSR %q: converting public key in CSR to TPM Public structure: %v", csr.Name, err)
		return false, nil
	}
	ok, err = attestData.AttestedCertifyInfo.Name.MatchesPublic(pub)
	if err != nil {
		glog.Infof("deny CSR %q: comparing ATTESTATION DATA to CSR public key: %v", csr.Name, err)
		return false, nil
	}
	if !ok {
		glog.Infof("deny CSR %q: ATTESTATION DATA doesn't match CSR public key", csr.Name)
		return false, nil
	}
	return true, nil
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

func clusterHasInstance(opts GCPConfig, zone string, instanceID uint64) (bool, error) {
	// zone looks like
	// "https://www.googleapis.com/compute/v1/projects/my-project/zones/us-central1-c"
	// Convert it to just "us-central1-c".
	zone = path.Base(zone)

	cluster, err := container.NewProjectsZonesClustersService(opts.Container).Get(opts.ProjectID, zone, opts.ClusterName).Do()
	if err != nil {
		return false, fmt.Errorf("fetching cluster info: %v", err)
	}
	for _, np := range cluster.NodePools {
		for _, ig := range np.InstanceGroupUrls {
			igName := path.Base(ig)
			ok, err := groupHasInstance(opts, zone, igName, instanceID)
			if err != nil {
				return false, fmt.Errorf("checking that group %q contains instance %v: %v", igName, instanceID, err)
			}
			if ok {
				return true, nil
			}
		}
	}
	return false, nil
}

func groupHasInstance(opts GCPConfig, zone, groupName string, instanceID uint64) (bool, error) {
	instances, err := compute.NewInstanceGroupManagersService(opts.Compute).ListManagedInstances(opts.ProjectID, zone, groupName).Do()
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

func isNotFound(err error) bool {
	gerr, ok := err.(*googleapi.Error)
	return ok && gerr.Code == http.StatusNotFound
}

func ensureNodeMatchesMetadataOrDelete(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest, metricsLabel string, client clientset.Interface) error {
	// TODO: short-circuit

	start := time.Now()
	if !strings.HasPrefix(x509cr.Subject.CommonName, "system:node:") {
		return nil
	}
	nodeName := strings.TrimPrefix(x509cr.Subject.CommonName, "system:node:")
	if len(nodeName) == 0 {
		return nil
	}

	node, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// if there is no existing Node object, return success
		return nil
	}
	if err != nil {
		// returning an error triggers a retry of this CSR.
		// if the errors are persistent, the CSR will not be approved and the node bootstrap will hang.
		return fmt.Errorf("error getting node %s: %v", nodeName, err)
	}

	delete, err := shouldDeleteNode(opts, node, getInstanceByName)
	if err != nil {
		// returning an error triggers a retry of this CSR.
		// if the errors are persistent, the CSR will not be approved and the node bootstrap will hang.
		return fmt.Errorf("error determining if node %s should be deleted: %v", nodeName, err)
	}
	if !delete {
		// if the existing Node object does not need to be removed, return success
		return nil
	}

	err = client.CoreV1().Nodes().Delete(nodeName, &metav1.DeleteOptions{Preconditions: metav1.NewUIDPreconditions(string(node.UID))})
	if apierrors.IsNotFound(err) {
		// If we wanted to delete and the node is gone, this counts as success
		return nil
	}
	if err != nil {
		// returning an error triggers a retry of this CSR.
		// if the errors are persistent, the CSR will not be approved and the node bootstrap will hang.
		return fmt.Errorf("error deleting node %s: %v", nodeName, err)
	}
	csrApprovalStatus.WithLabelValues("node_deleted", metricsLabel).Inc()
	csrApprovalLatency.WithLabelValues("node_deleted", metricsLabel).Observe(time.Since(start).Seconds())
	return nil
}

func shouldDeleteNode(opts GCPConfig, node *v1.Node, getInstance func(GCPConfig, string) (*compute.Instance, error)) (bool, error) {
	// Newly created node might not have pod CIDR allocated yet.
	if node.Spec.PodCIDR == "" {
		glog.V(2).Infof("Node %q has empty podCIDR.", node.Name)
		return false, nil
	}
	inst, err := getInstance(opts, node.Name)
	if err != nil {
		if err == instanceNotFound {
			glog.Warningf("Didn't find corresponding instance for node %q, will trigger node deletion.", node.Name)
			return true, nil
		}
		glog.Errorf("Error retrieving instance %q: %v", node.Name, err)
		return false, err
	}
	var unmatchedRanges []string
	for _, networkInterface := range inst.NetworkInterfaces {
		for _, r := range networkInterface.AliasIpRanges {
			if node.Spec.PodCIDR == r.IpCidrRange {
				glog.V(2).Infof("Instance %q has alias range that matches node's podCIDR.", inst.Name)
				return false, nil
			}
			unmatchedRanges = append(unmatchedRanges, r.IpCidrRange)
		}
	}
	if len(unmatchedRanges) != 0 {
		glog.Warningf("Instance %q has alias range(s) %v and none of them match node's podCIDR %s, will trigger node deletion.", inst.Name, unmatchedRanges, node.Spec.PodCIDR)
		return true, nil
	}
	// Instance with no alias range is route based, for which node object deletion is unnecessary.
	glog.V(2).Infof("Instance %q has no alias range.", inst.Name)
	return false, nil
}

var instanceNotFound = errors.New("instance not found")

func getInstanceByName(opts GCPConfig, instanceName string) (*compute.Instance, error) {
	srv := compute.NewInstancesService(opts.Compute)
	for _, z := range opts.Zones {
		inst, err := srv.Get(opts.ProjectID, z, instanceName).Do()
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return nil, err
		}
		return inst, nil
	}
	return nil, instanceNotFound
}
