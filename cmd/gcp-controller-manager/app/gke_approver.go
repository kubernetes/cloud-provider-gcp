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
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"strings"

	"github.com/google/go-tpm/tpm2"
	betacompute "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
	authorization "k8s.io/api/authorization/v1beta1"
	capi "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/app/csrmetrics"
	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
	"k8s.io/cloud-provider-gcp/pkg/tpmattest"
	"k8s.io/klog"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1beta1"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

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
	recordMetric := csrmetrics.ApprovalStartRecorder(authFlowLabelNone)
	if len(csr.Status.Certificate) != 0 {
		return nil
	}
	if approved, denied := certificates.GetCertApprovalCondition(&csr.Status); approved || denied {
		return nil
	}
	klog.Infof("approver got CSR %q", csr.Name)

	x509cr, err := certutil.ParseCSR(csr)
	if err != nil {
		recordMetric(csrmetrics.ApprovalStatusParseError)
		return fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}

	var tried []string
	for _, r := range a.validators {
		recordValidatorMetric := csrmetrics.ApprovalStartRecorder(r.authFlowLabel)
		if !r.recognize(a.opts, csr, x509cr) {
			continue
		}
		klog.Infof("validator %q: matched CSR %q", r.name, csr.Name)
		tried = append(tried, r.name)
		if r.validate != nil {
			ok, err := r.validate(a.opts, csr, x509cr)
			if err != nil {
				return fmt.Errorf("validating CSR %q: %v", csr.Name, err)
			}
			if !ok {
				klog.Infof("validator %q: denied CSR %q", r.name, csr.Name)
				recordValidatorMetric(csrmetrics.ApprovalStatusDeny)
				return a.updateCSR(csr, false, r.denyMsg)
			}
		}
		klog.Infof("CSR %q validation passed", csr.Name)

		approved, err := a.authorizeSAR(csr, r.permission)
		if err != nil {
			recordValidatorMetric(csrmetrics.ApprovalStatusSARError)
			return err
		}
		if !approved {
			klog.Warningf("validator %q: SubjectAccessReview denied for CSR %q", r.name, csr.Name)
			continue
		}
		klog.Infof("validator %q: SubjectAccessReview approved for CSR %q", r.name, csr.Name)
		if r.preApproveHook != nil {
			if err := r.preApproveHook(a.opts, csr, x509cr, r.authFlowLabel, a.client); err != nil {
				klog.Warningf("validator %q: preApproveHook failed for CSR %q: %v", r.name, csr.Name, err)
				recordValidatorMetric(csrmetrics.ApprovalStatusPreApproveHookError)
				return err
			}
			klog.Infof("validator %q: preApproveHook passed for CSR %q", r.name, csr.Name)
		}
		recordValidatorMetric(csrmetrics.ApprovalStatusApprove)
		return a.updateCSR(csr, true, r.approveMsg)
	}

	if len(tried) != 0 {
		recordMetric(csrmetrics.ApprovalStatusSARReject)
		return certificates.IgnorableError("recognized csr %q as %q but subject access review was not approved", csr.Name, tried)
	}
	klog.Infof("no validators matched CSR %q", csr.Name)
	recordMetric(csrmetrics.ApprovalStatusIgnore)
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
		klog.Infof("deny CSR %q: no SAN IPs", csr.Name)
		return false, nil
	case len(x509cr.EmailAddresses) > 0 || len(x509cr.URIs) > 0:
		klog.Infof("deny CSR %q: only DNS and IP SANs allowed", csr.Name)
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
			klog.Infof("deny CSR %q: IP addresses in CSR (%q) don't match NetworkInterfaces on instance %q (%+v)", csr.Name, x509cr.IPAddresses, instanceName, inst.NetworkInterfaces)
			return false, nil
		}
		return true, nil
	}
	klog.Infof("deny CSR %q: instance name %q doesn't match any VM in cluster project/zone", csr.Name, instanceName)
	return false, nil
}

var tpmAttestationBlocks = []string{
	"CERTIFICATE REQUEST",
	"VM IDENTITY",
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
		klog.Errorf("parsing csr.Spec.Request: %v", err)
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
		klog.Infof("deny CSR %q: parsing csr.Spec.Request: %v", csr.Name, err)
		return false, nil
	}
	attestDataRaw := blocks["ATTESTATION DATA"].Bytes
	attestSig := blocks["ATTESTATION SIGNATURE"].Bytes

	// TODO(awly): call ekPubAndIDFromCert instead of ekPubAndIDFromAPI when
	// ATTESTATION CERTIFICATE is reliably present in CSRs.
	aikPub, nodeID, err := ekPubAndIDFromAPI(opts, blocks)
	if err != nil {
		if _, ok := err.(temporaryError); ok {
			return false, fmt.Errorf("fetching EK public key from API: %v", err)
		}
		klog.Infof("deny CSR %q: fetching EK public key from API: %v", csr.Name, err)
		return false, nil
	}

	hostname := strings.TrimPrefix(x509cr.Subject.CommonName, "system:node:")
	if nodeID.Name != hostname {
		klog.Infof("deny CSR %q: VM name in ATTESTATION CERTIFICATE (%q) doesn't match CommonName in x509 CSR (%q)", csr.Name, nodeID.Name, x509cr.Subject.CommonName)
		return false, nil
	}
	if fmt.Sprint(nodeID.ProjectName) != opts.ProjectID {
		klog.Infof("deny CSR %q: received CSR for a different project Name (%q)", csr.Name, nodeID.ProjectName)
		return false, nil
	}

	recordMetric := csrmetrics.OutboundRPCStartRecorder("compute.InstancesService.Get")
	srv := compute.NewInstancesService(opts.Compute)
	inst, err := srv.Get(fmt.Sprint(nodeID.ProjectID), nodeID.Zone, nodeID.Name).Do()
	if err != nil {
		if isNotFound(err) {
			klog.Infof("deny CSR %q: VM doesn't exist in GCE API: %v", csr.Name, err)
			recordMetric(csrmetrics.OutboundRPCStatusNotFound)
			return false, nil
		}
		recordMetric(csrmetrics.OutboundRPCStatusError)
		return false, fmt.Errorf("fetching VM data from GCE API: %v", err)
	}
	recordMetric(csrmetrics.OutboundRPCStatusOK)
	if opts.VerifyClusterMembership {
		ok, err := clusterHasInstance(opts, inst.Zone, inst.Id)
		if err != nil {
			return false, fmt.Errorf("checking VM membership in cluster: %v", err)
		}
		if !ok {
			klog.Infof("deny CSR %q: VM %q doesn't belong to cluster %q", csr.Name, inst.Name, opts.ClusterName)
			return false, nil
		}
	}

	attestHash := sha256.Sum256(attestDataRaw)
	if err := rsa.VerifyPKCS1v15(aikPub, crypto.SHA256, attestHash[:], attestSig); err != nil {
		klog.Infof("deny CSR %q: verifying certification signature with AIK public key: %v", csr.Name, err)
		return false, nil
	}

	// Verify that attestDataRaw matches certificate.
	pub, err := tpmattest.MakePublic(x509cr.PublicKey)
	if err != nil {
		klog.Infof("deny CSR %q: converting public key in CSR to TPM Public structure: %v", csr.Name, err)
		return false, nil
	}
	attestData, err := tpm2.DecodeAttestationData(attestDataRaw)
	if err != nil {
		klog.Infof("deny CSR %q: parsing attestation data in CSR: %v", csr.Name, err)
		return false, nil
	}
	ok, err := attestData.AttestedCertifyInfo.Name.MatchesPublic(pub)
	if err != nil {
		klog.Infof("deny CSR %q: comparing ATTESTATION DATA to CSR public key: %v", csr.Name, err)
		return false, nil
	}
	if !ok {
		klog.Infof("deny CSR %q: ATTESTATION DATA doesn't match CSR public key", csr.Name)
		return false, nil
	}
	return true, nil
}

func ekPubAndIDFromCert(opts GCPConfig, blocks map[string]*pem.Block) (*rsa.PublicKey, *nodeidentity.Identity, error) {
	// When we switch away from ekPubAndIDFromAPI, remove this. Just a
	// safe-guard against accidental execution.
	panic("ekPubAndIDFromCert should not be reachable")

	attestCert := blocks["ATTESTATION CERTIFICATE"].Bytes
	aikCert, err := x509.ParseCertificate(attestCert)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing ATTESTATION_CERTIFICATE: %v", err)
	}
	if err := opts.TPMEndorsementCACache.verify(aikCert); err != nil {
		// TODO(awly): handle temporary CA unavailability without denying CSRs.
		return nil, nil, fmt.Errorf("verifying EK certificate validity: %v", err)
	}
	aikPub, ok := aikCert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("public key in ATTESTATION CERTIFICATE is %T, want *rsa.PublicKey", aikCert.PublicKey)
	}
	nodeID, err := nodeidentity.FromAIKCert(aikCert)
	if err != nil {
		return nil, nil, fmt.Errorf("failed extracting VM identity from EK certificate: %v", err)
	}

	return aikPub, &nodeID, nil
}

func ekPubAndIDFromAPI(opts GCPConfig, blocks map[string]*pem.Block) (*rsa.PublicKey, *nodeidentity.Identity, error) {
	nodeIDRaw := blocks["VM IDENTITY"].Bytes
	nodeID := new(nodeidentity.Identity)
	if err := json.Unmarshal(nodeIDRaw, nodeID); err != nil {
		return nil, nil, fmt.Errorf("failed parsing VM IDENTITY block: %v", err)
	}

	recordMetric := csrmetrics.OutboundRPCStartRecorder("compute.InstancesService.GetShieldedVmIdentity")
	srv := betacompute.NewInstancesService(opts.BetaCompute)
	resp, err := srv.GetShieldedVmIdentity(fmt.Sprint(nodeID.ProjectID), nodeID.Zone, nodeID.Name).Do()
	if err != nil {
		if isNotFound(err) {
			recordMetric(csrmetrics.OutboundRPCStatusNotFound)
			return nil, nil, fmt.Errorf("fetching Shielded VM identity: %v", err)
		}
		recordMetric(csrmetrics.OutboundRPCStatusError)
		return nil, nil, temporaryError(fmt.Errorf("fetching Shielded VM identity: %v", err))
	}
	recordMetric(csrmetrics.OutboundRPCStatusOK)
	if resp.SigningKey == nil {
		return nil, nil, fmt.Errorf("VM %q doesn't have a signing key in ShieldedVmIdentity", nodeID.Name)
	}
	block, _ := pem.Decode([]byte(resp.SigningKey.EkPub))
	if block == nil {
		return nil, nil, fmt.Errorf("failed parsing PEM block from EkPub %q", resp.SigningKey.EkPub)
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed parsing EK public key: %v", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("EK public key is %T, expected *rsa.PublickKey", pub)
	}
	return rsaPub, nodeID, nil
}

// temporaryError is used within validators to decide between hard-deny and
// temporary inability to validate.
type temporaryError error

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
	recordMetric := csrmetrics.OutboundRPCStartRecorder("container.ProjectsZonesClustersService.Get")
	cluster, err := container.NewProjectsZonesClustersService(opts.Container).Get(opts.ProjectID, zone, opts.ClusterName).Do()
	if err != nil {
		recordMetric(csrmetrics.OutboundRPCStatusError)
		return false, fmt.Errorf("fetching cluster info: %v", err)
	}
	recordMetric(csrmetrics.OutboundRPCStatusOK)
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
	recordMetric := csrmetrics.OutboundRPCStartRecorder("compute.InstanceGroupManagersService.ListManagedInstances")
	instances, err := compute.NewInstanceGroupManagersService(opts.Compute).ListManagedInstances(opts.ProjectID, zone, groupName).Do()
	if err != nil {
		recordMetric(csrmetrics.OutboundRPCStatusError)
		return false, err
	}
	recordMetric(csrmetrics.OutboundRPCStatusOK)
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
	recordMetric := csrmetrics.ApprovalStartRecorder(metricsLabel)
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
	recordMetric(csrmetrics.ApprovalStatusNodeDeleted)
	return nil
}

var errInstanceNotFound = errors.New("instance not found")

func shouldDeleteNode(opts GCPConfig, node *v1.Node, getInstance func(GCPConfig, string) (*compute.Instance, error)) (bool, error) {
	// Newly created node might not have pod CIDR allocated yet.
	if node.Spec.PodCIDR == "" {
		klog.V(2).Infof("Node %q has empty podCIDR.", node.Name)
		return false, nil
	}
	inst, err := getInstance(opts, node.Name)
	if err != nil {
		if err == errInstanceNotFound {
			klog.Warningf("Didn't find corresponding instance for node %q, will trigger node deletion.", node.Name)
			return true, nil
		}
		klog.Errorf("Error retrieving instance %q: %v", node.Name, err)
		return false, err
	}
	var unmatchedRanges []string
	for _, networkInterface := range inst.NetworkInterfaces {
		for _, r := range networkInterface.AliasIpRanges {
			if node.Spec.PodCIDR == r.IpCidrRange {
				klog.V(2).Infof("Instance %q has alias range that matches node's podCIDR.", inst.Name)
				return false, nil
			}
			unmatchedRanges = append(unmatchedRanges, r.IpCidrRange)
		}
	}
	if len(unmatchedRanges) != 0 {
		klog.Warningf("Instance %q has alias range(s) %v and none of them match node's podCIDR %s, will trigger node deletion.", inst.Name, unmatchedRanges, node.Spec.PodCIDR)
		return true, nil
	}
	// Instance with no alias range is route based, for which node object deletion is unnecessary.
	klog.V(2).Infof("Instance %q has no alias range.", inst.Name)
	return false, nil
}

func getInstanceByName(opts GCPConfig, instanceName string) (*compute.Instance, error) {
	srv := compute.NewInstancesService(opts.Compute)
	for _, z := range opts.Zones {
		recordMetric := csrmetrics.OutboundRPCStartRecorder("compute.InstancesService.Get")
		inst, err := srv.Get(opts.ProjectID, z, instanceName).Do()
		if err != nil {
			if isNotFound(err) {
				recordMetric(csrmetrics.OutboundRPCStatusNotFound)
				continue
			}
			recordMetric(csrmetrics.OutboundRPCStatusError)
			return nil, err
		}
		recordMetric(csrmetrics.OutboundRPCStatusOK)
		return inst, nil
	}
	return nil, errInstanceNotFound
}
