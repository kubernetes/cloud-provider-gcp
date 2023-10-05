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
	"net"
	"net/http"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-tpm/tpm2"
	betacompute "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
	"k8s.io/apimachinery/pkg/util/wait"

	authorization "k8s.io/api/authorization/v1"
	capi "k8s.io/api/certificates/v1"
	certsv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apiserver/pkg/util/webhook"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/cloud-provider-gcp/pkg/csrmetrics"
	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
	"k8s.io/cloud-provider-gcp/pkg/tpmattest"
	"k8s.io/klog/v2"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1"
	"k8s.io/kubernetes/pkg/controller/certificates"
	"k8s.io/kubernetes/pkg/features"
)

const (
	legacyKubeletUsername = "kubelet"
	tpmKubeletUsername    = "kubelet-bootstrap"

	authFlowLabelNone = "unknown"

	createdByInstanceMetadataKey = "created-by"
)

var (
	// For the first startupErrorsThreshold after startupTime, label SAR errors
	// differently.
	// When kube-apiserver starts up (around the same time), it takes some time
	// to setup RBAC rules for these SAR checks. This special labeling of
	// errors lets us filter out the (expected) error noise at master startup.
	//
	// Note: this ignores leader election. We only want to give kube-apiserver
	// some time after cluster startup to initialize RBAC rules.
	startupTime            = time.Now()
	startupErrorsThreshold = 5 * time.Minute
)

// nodeApprover handles approval/denial of CSRs based on SubjectAccessReview and
// CSR attestation data.
type nodeApprover struct {
	ctx        *controllerContext
	validators []csrValidator
}

func newNodeApprover(ctx *controllerContext) *nodeApprover {
	return &nodeApprover{
		ctx:        ctx,
		validators: csrValidators(ctx),
	}
}

func csrValidators(ctx *controllerContext) []csrValidator {
	// More specific validators go first.
	validators := []csrValidator{
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
			name:          "kubelet server certificate SubjectAccessReview",
			authFlowLabel: "kubelet_server_self",
			recognize:     isNodeServerCert,
			validate:      validateNodeServerCert,
			permission:    authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "selfnodeclient"},
			approveMsg:    "Auto approving kubelet server certificate after SubjectAccessReview.",
		},
	}
	if ctx.csrApproverAllowLegacyKubelet {
		validators = append(validators, csrValidator{
			name:          "kubelet client certificate SubjectAccessReview",
			authFlowLabel: "kubelet_client_legacy",
			recognize:     isLegacyNodeClientCert,
			permission:    authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "nodeclient"},
			approveMsg:    "Auto approving kubelet client certificate after SubjectAccessReview.",

			preApproveHook: ensureNodeMatchesMetadataOrDelete,
		})
	}
	return validators
}

func (a *nodeApprover) handle(ctx context.Context, csr *capi.CertificateSigningRequest) error {
	recordMetric := csrmetrics.ApprovalStartRecorder(authFlowLabelNone)
	if len(csr.Status.Certificate) != 0 {
		return nil
	}
	if approved, denied := certificates.GetCertApprovalCondition(&csr.Status); approved || denied {
		return nil
	}
	klog.Infof("approver got CSR %q", csr.Name)

	x509cr, err := certutil.ParseCSR(csr.Spec.Request)
	if err != nil {
		recordMetric(csrmetrics.ApprovalStatusParseError)
		return fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}

	for _, r := range a.validators {
		recordValidatorMetric := csrmetrics.ApprovalStartRecorder(r.authFlowLabel)
		if !r.recognize(csr, x509cr) {
			continue
		}
		klog.Infof("validator %q: matched CSR %q", r.name, csr.Name)
		if r.validate != nil {
			ok, err := r.validate(a.ctx, csr, x509cr)
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
			if time.Since(startupTime) < startupErrorsThreshold {
				recordValidatorMetric(csrmetrics.ApprovalStatusSARErrorAtStartup)
			} else {
				recordValidatorMetric(csrmetrics.ApprovalStatusSARError)
			}
			return err
		}
		if !approved {
			if time.Since(startupTime) < startupErrorsThreshold {
				recordValidatorMetric(csrmetrics.ApprovalStatusSARRejectAtStartup)
			} else {
				recordValidatorMetric(csrmetrics.ApprovalStatusSARReject)
			}
			return certificates.IgnorableError("recognized csr %q as %q but subject access review was not approved", csr.Name, r.name)
		}
		klog.Infof("validator %q: SubjectAccessReview approved for CSR %q", r.name, csr.Name)
		if r.preApproveHook != nil {
			if err := r.preApproveHook(a.ctx, csr, x509cr); err != nil {
				klog.Warningf("validator %q: preApproveHook failed for CSR %q: %v", r.name, csr.Name, err)
				recordValidatorMetric(csrmetrics.ApprovalStatusPreApproveHookError)
				return err
			}
			klog.Infof("validator %q: preApproveHook passed for CSR %q", r.name, csr.Name)
		}
		recordValidatorMetric(csrmetrics.ApprovalStatusApprove)
		return a.updateCSR(csr, true, r.approveMsg)
	}

	klog.Infof("no validators matched CSR %q", csr.Name)
	recordMetric(csrmetrics.ApprovalStatusIgnore)
	return nil
}

func (a *nodeApprover) updateCSR(csr *capi.CertificateSigningRequest, approved bool, msg string) error {
	if approved {
		csr.Status.Conditions = append(csr.Status.Conditions, capi.CertificateSigningRequestCondition{
			Type:    capi.CertificateApproved,
			Reason:  "AutoApproved",
			Message: msg,
			Status:  v1.ConditionTrue,
		})
	} else {
		csr.Status.Conditions = append(csr.Status.Conditions, capi.CertificateSigningRequestCondition{
			Type:    capi.CertificateDenied,
			Reason:  "AutoDenied",
			Message: msg,
			Status:  v1.ConditionTrue,
		})
	}
	updateRecordMetric := csrmetrics.OutboundRPCStartRecorder("k8s.CertificateSigningRequests.updateApproval")
	_, err := a.ctx.client.CertificatesV1().CertificateSigningRequests().UpdateApproval(context.TODO(), csr.Name, csr, metav1.UpdateOptions{})
	if err != nil {
		updateRecordMetric(csrmetrics.OutboundRPCStatusError)
		return fmt.Errorf("error updating approval status for csr: %v", err)
	}
	updateRecordMetric(csrmetrics.OutboundRPCStatusOK)
	return nil
}

type recognizeFunc func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool
type validateFunc func(ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error)
type preApproveHookFunc func(ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) error

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

func (a *nodeApprover) authorizeSAR(csr *capi.CertificateSigningRequest, rattrs authorization.ResourceAttributes) (bool, error) {
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
	sar, err := a.ctx.client.AuthorizationV1().SubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
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

	// see https://issue.k8s.io/109077
	kubeletClientUsagesNoEncipherment = []capi.KeyUsage{
		capi.UsageDigitalSignature,
		capi.UsageClientAuth,
	}

	kubeletServerUsages = []capi.KeyUsage{
		capi.UsageKeyEncipherment,
		capi.UsageDigitalSignature,
		capi.UsageServerAuth,
	}

	// see https://issue.k8s.io/109077
	kubeletServerUsagesNoEncipherment = []capi.KeyUsage{
		capi.UsageDigitalSignature,
		capi.UsageServerAuth,
	}
)

func isNodeCert(_ *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !reflect.DeepEqual([]string{"system:nodes"}, x509cr.Subject.Organization) {
		return false
	}
	if len(x509cr.EmailAddresses) > 0 || len(x509cr.URIs) > 0 {
		return false
	}
	return strings.HasPrefix(x509cr.Subject.CommonName, "system:node:")
}

func isNodeClientCert(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeCert(csr, x509cr) {
		return false
	}
	if csr.Spec.SignerName != certsv1.KubeAPIServerClientKubeletSignerName {
		return false
	}
	if len(x509cr.DNSNames) > 0 || len(x509cr.IPAddresses) > 0 {
		return false
	}
	return hasExactUsages(csr, kubeletClientUsagesNoEncipherment) || hasExactUsages(csr, kubeletClientUsages)
}

func isLegacyNodeClientCert(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeClientCert(csr, x509cr) {
		return false
	}
	return csr.Spec.Username == legacyKubeletUsername
}

func isNodeServerCert(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeCert(csr, x509cr) {
		return false
	}
	if csr.Spec.SignerName != certsv1.KubeletServingSignerName {
		return false
	}
	if !hasExactUsages(csr, kubeletServerUsagesNoEncipherment) && !hasExactUsages(csr, kubeletServerUsages) {
		return false
	}
	return csr.Spec.Username == x509cr.Subject.CommonName
}

// Only check that IPs in SAN match an existing VM in the project.
// Username was already checked against CN, so this CSR is coming from
// authenticated kubelet.
func validateNodeServerCert(ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
	switch {
	case len(x509cr.IPAddresses) == 0:
		klog.Infof("deny CSR %q: no SAN IPs", csr.Name)
		return false, nil
	case len(x509cr.EmailAddresses) > 0 || len(x509cr.URIs) > 0:
		klog.Infof("deny CSR %q: only DNS and IP SANs allowed", csr.Name)
		return false, nil
	}

	srv := compute.NewInstancesService(ctx.gcpCfg.Compute)
	instanceName := strings.TrimPrefix(csr.Spec.Username, "system:node:")
	for _, z := range ctx.gcpCfg.Zones {
		inst, err := srv.Get(ctx.gcpCfg.ProjectID, z, instanceName).Do()
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return false, err
		}

		// Format the Domain-scoped projectID before validating the DNS name, e.g. example.com:my-project-123456789012
		projectID := ctx.gcpCfg.ProjectID
		if strings.Contains(projectID, ":") {
			parts := strings.Split(projectID, ":")
			if len(parts) != 2 {
				klog.Infof("expected the Domain-scoped project to contain only one colon, got: %s", projectID)
				return false, err
			}
			projectID = fmt.Sprintf("%s.%s", parts[1], parts[0])
		}

		for _, dns := range x509cr.DNSNames {
			// Linux DNSName should be as the format of [INSTANCE_NAME].c.[PROJECT_ID].internal when using the global DNS, and [INSTANCE_NAME].[ZONE].c.[PROJECT_ID].internal when using zonal DNS.
			// Windows DNSName should be INSTANCE_NAME
			if dns != instanceName && dns != fmt.Sprintf("%s.c.%s.internal", instanceName, projectID) && dns != fmt.Sprintf("%s.%s.c.%s.internal", instanceName, z, projectID) {
				klog.Infof("deny CSR %q: DNSName in CSR (%q) doesn't match default DNS format on instance %q", csr.Name, dns, instanceName)
				return false, nil
			}
		}
		instIps := getInstanceIps(inst.NetworkInterfaces)
	scanIPs:
		for _, ip := range x509cr.IPAddresses {
			for _, instIP := range instIps {
				if ip.Equal(net.ParseIP(instIP)) {
					continue scanIPs
				}
			}
			klog.Infof("deny CSR %q: IP addresses in CSR (%q) don't match NetworkInterfaces on instance %q (%+v)", csr.Name, x509cr.IPAddresses, instanceName, instIps)
			return false, nil
		}
		return true, nil
	}
	klog.Infof("deny CSR %q: instance name %q doesn't match any VM in cluster project/zone", csr.Name, instanceName)
	return false, nil
}

func getInstanceIps(ifaces []*compute.NetworkInterface) []string {
	var ips []string
	for _, iface := range ifaces {
		ips = append(ips, iface.NetworkIP)
		if iface.Ipv6Address != "" {
			ips = append(ips, iface.Ipv6Address)
		}
		for _, ac := range iface.AccessConfigs {
			if ac.NatIP != "" {
				ips = append(ips, ac.NatIP)
			}
		}
		for _, ac := range iface.Ipv6AccessConfigs {
			if ac.ExternalIpv6 != "" {
				ips = append(ips, ac.ExternalIpv6)
			}
		}
	}
	return ips
}

var tpmAttestationBlocks = []string{
	"CERTIFICATE REQUEST",
	"VM IDENTITY",
	"ATTESTATION DATA",
	"ATTESTATION SIGNATURE",
}

func isNodeClientCertWithAttestation(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeClientCert(csr, x509cr) {
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

func getInstanceMetadata(inst *compute.Instance, key string) string {
	if inst == nil || inst.Metadata == nil || inst.Metadata.Items == nil {
		return ""
	}

	for _, item := range inst.Metadata.Items {
		if item.Key == key && item.Value != nil {
			return *item.Value
		}
	}
	return ""
}

func validateTPMAttestation(ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
	blocks, err := parsePEMBlocks(csr.Spec.Request)
	if err != nil {
		klog.Infof("deny CSR %q: parsing csr.Spec.Request: %v", csr.Name, err)
		return false, nil
	}
	attestDataRaw := blocks["ATTESTATION DATA"].Bytes
	attestSig := blocks["ATTESTATION SIGNATURE"].Bytes

	// TODO(awly): call ekPubAndIDFromCert instead of ekPubAndIDFromAPI when
	// ATTESTATION CERTIFICATE is reliably present in CSRs.
	aikPub, nodeID, err := ekPubAndIDFromAPI(ctx, blocks)
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
	if fmt.Sprint(nodeID.ProjectName) != ctx.gcpCfg.ProjectID {
		klog.Infof("deny CSR %q: received CSR for a different project Name (%q)", csr.Name, nodeID.ProjectName)
		return false, nil
	}

	recordMetric := csrmetrics.OutboundRPCStartRecorder("compute.InstancesService.Get")
	srv := compute.NewInstancesService(ctx.gcpCfg.Compute)
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
	if ctx.csrApproverVerifyClusterMembership {
		// get the instance group of this instance from the metadata.
		// the metadata is user controlled, clusterHasInstance verifies
		// that the group is indeed part of the cluster.
		instanceGroupHint := getInstanceMetadata(inst, createdByInstanceMetadataKey)
		klog.V(3).Infof("inst[%d] has instanceGroupHint %q", inst.Id, instanceGroupHint)
		ok, err := clusterHasInstance(ctx, inst, instanceGroupHint)
		if err != nil {
			return false, fmt.Errorf("checking VM membership in cluster: %v", err)
		}
		if !ok {
			klog.Infof("deny CSR %q: VM %q doesn't belong to cluster %q", csr.Name, inst.Name, ctx.gcpCfg.ClusterName)
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

// Delete func ekPubAndIDFromCert(#200)

func ekPubAndIDFromAPI(ctx *controllerContext, blocks map[string]*pem.Block) (*rsa.PublicKey, *nodeidentity.Identity, error) {
	nodeIDRaw := blocks["VM IDENTITY"].Bytes
	nodeID := new(nodeidentity.Identity)
	if err := json.Unmarshal(nodeIDRaw, nodeID); err != nil {
		return nil, nil, fmt.Errorf("failed parsing VM IDENTITY block: %v", err)
	}

	recordMetric := csrmetrics.OutboundRPCStartRecorder("compute.InstancesService.GetShieldedVmIdentity")
	srv := betacompute.NewInstancesService(ctx.gcpCfg.BetaCompute)
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

// getClusterInstanceGroupUrls returns a list of instance groups for all node pools in the cluster.
func getClusterInstanceGroupUrls(ctx *controllerContext) ([]string, error) {
	var instanceGroupUrls []string
	clusterName := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", ctx.gcpCfg.ProjectID, ctx.gcpCfg.Location, ctx.gcpCfg.ClusterName)

	recordMetric := csrmetrics.OutboundRPCStartRecorder("container.ProjectsLocationsClustersService.Get")
	cluster, err := container.NewProjectsLocationsClustersService(ctx.gcpCfg.Container).Get(clusterName).Do()
	if err != nil {
		recordMetric(csrmetrics.OutboundRPCStatusError)
		return nil, fmt.Errorf("fetching cluster info: %v", err)
	}

	recordMetric(csrmetrics.OutboundRPCStatusOK)
	for _, np := range cluster.NodePools {
		instanceGroupUrls = append(instanceGroupUrls, np.InstanceGroupUrls...)
	}
	return instanceGroupUrls, nil
}

// InstanceGroupHint is the name of the instancegroup obtained from the instance metadata.
// Since this is user-modifiable, we should still verify membership.
// However, we can avoid some GCE API ListManagedInstanceGroupInstances calls using the hint.

// Verify that the instanceGroupHint is in fact
// part of the cluster's known instance groups
//
// if it is: return the resolved instanceGroup
// else: ""
func validateInstanceGroupHint(instanceGroupUrls []string, instanceGroupHint string) (string, error) {
	if instanceGroupHint == "" {
		return "", fmt.Errorf("validateInstanceGroupHint: hint is empty")
	}

	igName, location, err := parseInstanceGroupURL(instanceGroupHint)
	if err != nil {
		return "", err
	}

	var resolved string
	for _, g := range instanceGroupUrls {
		gn, gl, err := parseInstanceGroupURL(g)
		if err != nil {
			return "", err
		}
		if gl == location && gn == igName {
			resolved = g
		}
	}

	if resolved == "" {
		return "", fmt.Errorf("hinted instance group %q not found in cluster", instanceGroupHint)
	}

	return resolved, nil
}

var errNotFoundListReferrers = errors.New("not found the entry in ListReferrers")

func checkInstanceReferrersBackOff(ctx *controllerContext, instance *compute.Instance, clusterInstanceGroupUrls []string) bool {
	if !ctx.csrApproverListReferrersConfig.enabled {
		return false
	}
	klog.Infof("Using compute.InstancesService.ListReferrers to verify cluster membership of instance %q", instance.Name)

	var found bool
	startTime := time.Now()
	backoffPolicy := wait.Backoff{
		Duration: ctx.csrApproverListReferrersConfig.initialInterval,
		Factor:   1.5,
		Jitter:   0.2,
		Steps:    ctx.csrApproverListReferrersConfig.retryCount,
	}
	webhook.WithExponentialBackoff(context.TODO(), backoffPolicy, func() error {
		var retryErr error
		found, retryErr = checkInstanceReferrers(ctx, instance, clusterInstanceGroupUrls)
		if retryErr != nil || !found {
			return errNotFoundListReferrers
		}
		return nil
	},
		func(err error) bool {
			return err != nil
		},
	)

	if found {
		klog.V(2).Infof("Determined cluster membership of instance %q using compute.InstancesService.ListReferrers after %v", instance.Name, time.Since(startTime))
	} else {
		klog.Warningf("Could not determine cluster membership of instance %q using compute.InstancesService.ListReferrers after %v; falling back to checking all instance groups", instance.Name, time.Since(startTime))
	}
	return found
}

func checkInstanceReferrers(ctx *controllerContext, instance *compute.Instance, clusterInstanceGroupUrls []string) (bool, error) {
	// instanceZone looks like
	// "https://www.googleapis.com/compute/v1/projects/my-project/zones/us-central1-c"
	// Extract the bare zone name just "us-central1-c".
	instanceZoneName := path.Base(instance.Zone)
	clusterInstanceGroupMap := map[string]bool{}
	for _, ig := range clusterInstanceGroupUrls {
		// GKE's cluster.NodePools[].instanceGroupUrls are of the form:
		// https://www.googleapis.com/compute/v1/projects/my-project/zones/us-central1-c/instanceGroupManagers/instance-group-1.
		// Where as instance referrers are of the form:
		// https://www.googleapis.com/compute/v1/projects/my-project/zones/us-central1-c/instanceGroups/instance-group-1.
		// With the string replace below we convert them to the same form.
		clusterInstanceGroupMap[strings.Replace(ig, "/instanceGroupManagers/", "/instanceGroups/", 1)] = true
	}

	filter := func(referres *compute.InstanceListReferrers) error {
		for _, referrer := range referres.Items {
			if clusterInstanceGroupMap[referrer.Referrer] {
				return &foundError{}
			}
		}
		return nil
	}

	recordMetric := csrmetrics.OutboundRPCStartRecorder("compute.InstancesService.ListReferrers")
	err := compute.NewInstancesService(ctx.gcpCfg.Compute).ListReferrers(ctx.gcpCfg.ProjectID, instanceZoneName, instance.Name).Pages(context.TODO(), filter)
	if err != nil {
		switch err.(type) {
		case *foundError:
			klog.Infof("found matching instance group using compute.InstancesService.ListReferrers for instance %q", instance.Name)
			recordMetric(csrmetrics.OutboundRPCStatusOK)
			return true, nil
		default:
			recordMetric(csrmetrics.OutboundRPCStatusError)
			return false, err
		}
	}

	recordMetric(csrmetrics.OutboundRPCStatusOK)
	return false, nil
}

func clusterHasInstance(ctx *controllerContext, instance *compute.Instance, instanceGroupHint string) (bool, error) {
	clusterInstanceGroupUrls, err := getClusterInstanceGroupUrls(ctx)
	if err != nil {
		return false, err
	}

	ok := checkInstanceReferrersBackOff(ctx, instance, clusterInstanceGroupUrls)
	if ok {
		return true, nil
	}

	validatedInstanceGroupHint, err := validateInstanceGroupHint(clusterInstanceGroupUrls, instanceGroupHint)
	if err != nil {
		klog.Warningf("error validating instance group: %v", err)
	} else {
		clusterInstanceGroupUrls = append([]string{validatedInstanceGroupHint}, clusterInstanceGroupUrls...)
	}

	klog.V(3).Infof("clusterInstanceGroupUrls %+v", clusterInstanceGroupUrls)

	// instanceZone looks like
	// "https://www.googleapis.com/compute/v1/projects/my-project/zones/us-central1-c"
	// Extract the bare zone name just "us-central1-c".
	instanceZoneName := path.Base(instance.Zone)

	var errors []error

	for _, ig := range clusterInstanceGroupUrls {
		igName, igLocation, err := parseInstanceGroupURL(ig)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		// InstanceGroups can be regional, igLocation can be either region
		// or a zone. Match them to instanceZone by prefix to cover both.
		if !strings.HasPrefix(instanceZoneName, igLocation) {
			klog.V(2).Infof("instance group %q is in zone/region %q, node sending the CSR is in %q; skipping instance group", ig, igLocation, instanceZoneName)
			continue
		}

		// Note: use igLocation here instead of instanceZone.
		// InstanceGroups can be regional, instances are always zonal.
		ok, err := groupHasInstance(ctx, igLocation, igName, instance.Id)
		if err != nil {
			errors = append(errors, fmt.Errorf("checking that group %q contains instance %v: %v", igName, instance.Id, err))
			continue
		}
		if ok {
			return true, nil
		}
	}

	if len(errors) > 0 {
		return false, fmt.Errorf("clusterHasInstance failed: %q", errors)
	}

	return false, nil
}

type foundError struct{}

func (*foundError) Error() string {
	return "found"
}

func groupHasInstance(ctx *controllerContext, groupLocation, groupName string, instanceID uint64) (bool, error) {
	recordMetric := csrmetrics.OutboundRPCStartRecorder("compute.InstanceGroupManagersService.ListManagedInstances")
	filter := func(response *compute.InstanceGroupManagersListManagedInstancesResponse) error {
		for _, instance := range response.ManagedInstances {
			// If the instance is found we return foundError which allows us to exit early and
			// not go through the rest of the pages. The ListManagedInstances call does not
			// support filtering so we have to resort to this hack.
			if instance.Id == instanceID {
				return &foundError{}
			}
		}
		return nil
	}
	err := compute.NewInstanceGroupManagersService(ctx.gcpCfg.Compute).ListManagedInstances(ctx.gcpCfg.ProjectID, groupLocation, groupName).Pages(context.TODO(), filter)
	if err != nil {
		switch err.(type) {
		case *foundError:
			recordMetric(csrmetrics.OutboundRPCStatusOK)
			return true, nil
		default:
			recordMetric(csrmetrics.OutboundRPCStatusError)
			return false, err
		}

	}
	recordMetric(csrmetrics.OutboundRPCStatusOK)
	return false, nil
}

func parseInstanceGroupURL(ig string) (name, location string, err error) {
	igParts := strings.Split(ig, "/")
	if len(igParts) < 4 {
		return "", "", fmt.Errorf("instance group URL is invalid %q; expect a URL with zone and instance group names", ig)
	}
	name = igParts[len(igParts)-1]
	location = igParts[len(igParts)-3]
	return name, location, nil
}

func isNotFound(err error) bool {
	gerr, ok := err.(*googleapi.Error)
	return ok && gerr.Code == http.StatusNotFound
}

func ensureNodeMatchesMetadataOrDelete(ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) error {
	// TODO: short-circuit
	if !strings.HasPrefix(x509cr.Subject.CommonName, "system:node:") {
		return nil
	}
	nodeName := strings.TrimPrefix(x509cr.Subject.CommonName, "system:node:")
	if len(nodeName) == 0 {
		return nil
	}

	recordMetric := csrmetrics.OutboundRPCStartRecorder("k8s.Nodes.get")
	node, err := ctx.client.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		recordMetric(csrmetrics.OutboundRPCStatusNotFound)
		// GCE MIGs currently reuse the instance name on new VMs. For example,
		// after GCE Preemptible/Spot VM is preempted, the instance started by
		// MIG will have the same instance name. This can result in old "stale"
		// pods still bound to the node to exist after a node is preempted. In
		// some cases, during preemption, the GCE instance is deleted and cloud
		// controller will sync with k8s and the underlying k8s node object will
		// be deleted
		// (https://github.com/kubernetes/kubernetes/blob/44e403f5bbc71eb5f577da6ac8a2e29875ac1d28/staging/src/k8s.io/cloud-provider/controllers/nodelifecycle/node_lifecycle_controller.go#L147-L153).
		// However it is possible that the pods bound to this node will not be
		// garbage collected as the orphaned pods
		// (https://github.com/kubernetes/kubernetes/blob/1ab40212a4e6cb10b3ae88c2e6c912a9fc1b1605/pkg/controller/podgc/gc_controller.go#L220-L255)
		// will only be cleared periodically every 20 seconds. The 20 seconds
		// check can race with the time the new node is started. If the new node
		// is started before the pod GC will run, the pods from the previous
		// node will not be cleared. To avoid this situation, explicitly delete
		// all pods bound to the node name, even if the node object does not
		// exist.
		if ctx.clearStalePodsOnNodeRegistration {
			if err := deleteAllPodsBoundToNode(ctx, nodeName); err != nil {
				klog.Warningf("Failed to delete all pods bound to node %q: %v", nodeName, err)
			}
		}
		return nil
	}
	if err != nil {
		recordMetric(csrmetrics.OutboundRPCStatusError)
		// returning an error triggers a retry of this CSR.
		// if the errors are persistent, the CSR will not be approved and the node bootstrap will hang.
		return fmt.Errorf("error getting node %s: %v", nodeName, err)
	}
	recordMetric(csrmetrics.OutboundRPCStatusOK)

	delete, err := shouldDeleteNode(ctx, node, getInstanceByName)
	if err != nil {
		// returning an error triggers a retry of this CSR.
		// if the errors are persistent, the CSR will not be approved and the node bootstrap will hang.
		return fmt.Errorf("error determining if node %s should be deleted: %v", nodeName, err)
	}
	if !delete {
		// if the existing Node object does not need to be removed, return success
		return nil
	}

	recordMetric = csrmetrics.OutboundRPCStartRecorder("k8s.Nodes.delete")
	err = ctx.client.CoreV1().Nodes().Delete(context.TODO(), nodeName, metav1.DeleteOptions{Preconditions: metav1.NewUIDPreconditions(string(node.UID))})
	// Pod Deletion is best effort, do not block CSR approval during node
	// registration if there was an issue deleting pods on the node object GCE
	// MIGs currently reuse the instance name on new VMs. For example, after GCE
	// Preemptible/Spot VM is preempted, the instance started by MIG will have
	// the same instance name. This can result in old "stale" pods still bound
	// to the node to exist after a node is preempted. In the case that a GCE
	// node is preempted and new GCE instance is created with the same name,
	// explicitly delete all bounds to the old node.
	if ctx.clearStalePodsOnNodeRegistration {
		if err := deleteAllPodsBoundToNode(ctx, nodeName); err != nil {
			klog.Warningf("Failed to delete all pods bound to node %q: %v", nodeName, err)
		}
	}

	if apierrors.IsNotFound(err) {
		recordMetric(csrmetrics.OutboundRPCStatusNotFound)
		// If we wanted to delete and the node is gone, this counts as success
		return nil
	}
	if err != nil {
		recordMetric(csrmetrics.OutboundRPCStatusError)
		// returning an error triggers a retry of this CSR.
		// if the errors are persistent, the CSR will not be approved and the node bootstrap will hang.
		return fmt.Errorf("error deleting node %s: %v", nodeName, err)
	}
	recordMetric(csrmetrics.OutboundRPCStatusOK)
	return nil
}

var errInstanceNotFound = errors.New("instance not found")

func shouldDeleteNode(ctx *controllerContext, node *v1.Node, getInstance func(*controllerContext, string) (*compute.Instance, error)) (bool, error) {
	inst, err := getInstance(ctx, node.Name)
	if err != nil {
		if err == errInstanceNotFound {
			klog.Warningf("Didn't find corresponding instance for node %q, will trigger node deletion.", node.Name)
			return true, nil
		}
		klog.Errorf("Error retrieving instance %q: %v", node.Name, err)
		return false, err
	}
	if ctx.clearStalePodsOnNodeRegistration {
		oldInstanceID := node.ObjectMeta.Annotations[InstanceIDAnnotationKey]
		newInstanceID := strconv.FormatUint(inst.Id, 10)
		// Even if a GCE Instance reuses the instance name, the underlying GCE instance will change (for example on Preemptible / Spot VMs during preemption).
		if oldInstanceID != "" && newInstanceID != "" && oldInstanceID != newInstanceID {
			klog.Infof("Detected change in instance ID on node %q - Old Instance ID: %q ; New Instance ID: %q", inst.Name, oldInstanceID, newInstanceID)
			return true, nil
		}
	}
	return false, nil
}

func deleteAllPodsBoundToNode(ctx *controllerContext, nodeName string) error {
	deletedPods := []string{}

	podList, err := ctx.client.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(),
		metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + nodeName,
			// Note: We pass ResourceVersion=0, to ensure that pod list is fetched from the api-server cache as opposed to going to etcd.
			// xref: https://github.com/kubernetes/kubernetes/blob/ef8c4fbca8e5bed1e7edc162b95c412a7f1a758e/staging/src/k8s.io/apiserver/pkg/storage/cacher/cacher.go#L621
			// This results in lower resource usage on the api-server when many nodes are registering in parallel.
			ResourceVersion: "0",
		},
	)
	if err != nil {
		return fmt.Errorf("failed to list pods on node: %v: %v", nodeName, err)
	}

	var errs []error
	for _, pod := range podList.Items {
		err := markFailedAndDeletePodWithCondition(ctx, &pod)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		deletedPods = append(deletedPods, pod.Name)
	}
	if len(deletedPods) > 0 {
		klog.Infof("Pods %s bound to node %s were deleted.", strings.Join(deletedPods[:], ", "), nodeName)
	}
	return utilerrors.NewAggregate(errs)
}

func markFailedAndDeletePodWithCondition(ctx *controllerContext, pod *v1.Pod) error {
	// Based on pod gc controller (https://github.com/kubernetes/kubernetes/blob/3833c0c349b53f08b4e063065d848854837b743c/pkg/controller/podgc/gc_controller.go)

	const fieldManager = "GCPControllerManager"
	if utilfeature.DefaultFeatureGate.Enabled(features.PodDisruptionConditions) {
		// Mark the pod as failed - this is especially important in case the pod
		// is orphaned, in which case the pod would remain in the Running phase
		// forever as there is no kubelet running to change the phase.
		if pod.Status.Phase != v1.PodSucceeded && pod.Status.Phase != v1.PodFailed {
			podApply := corev1apply.Pod(pod.Name, pod.Namespace).WithStatus(corev1apply.PodStatus())
			// we don't need to extract the pod apply configuration and can send
			// only phase and the DisruptionTarget condition as GCPControllerManager would not
			// own other fields. If the DisruptionTarget condition is owned by
			// GCPControllerManager it means that it is in the Failed phase, so sending the
			// condition will not be re-attempted.
			podApply.Status.WithPhase(v1.PodFailed)
			podApply.Status.WithConditions(
				corev1apply.PodCondition().
					WithType(v1.DisruptionTarget).
					WithStatus(v1.ConditionTrue).
					WithReason("DeletionByGCPControllerManager").
					WithMessage(fmt.Sprintf("%s: node no longer exists", fieldManager)).
					WithLastTransitionTime(metav1.Now()))

			if _, err := ctx.client.CoreV1().Pods(pod.Namespace).ApplyStatus(context.TODO(), podApply, metav1.ApplyOptions{FieldManager: fieldManager, Force: true}); err != nil {
				return err
			}
		}
	}
	return ctx.client.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, *metav1.NewDeleteOptions(0))
}

func getInstanceByName(ctx *controllerContext, instanceName string) (*compute.Instance, error) {
	srv := compute.NewInstancesService(ctx.gcpCfg.Compute)
	for _, z := range ctx.gcpCfg.Zones {
		recordMetric := csrmetrics.OutboundRPCStartRecorder("compute.InstancesService.Get")
		inst, err := srv.Get(ctx.gcpCfg.ProjectID, z, instanceName).Do()
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
