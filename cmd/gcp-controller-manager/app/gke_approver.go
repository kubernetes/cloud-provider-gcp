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
	"golang.org/x/oauth2"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	authorization "k8s.io/api/authorization/v1beta1"
	capi "k8s.io/api/certificates/v1beta1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
	"k8s.io/cloud-provider-gcp/pkg/tpmattest"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1beta1"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

var (
	csrApprovalStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "csr_approvals",
		Help: "Count of approved, denied and ignored CSRs",
	}, []string{"status", "kind"})
	csrApprovalLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "csr_approval_latency_seconds",
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

	// TODO(awly): eventually this should be true. But that makes testing
	// harder, so disabling or now.
	validateClusterMembership = false

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
		if r.validate != nil && !r.validate(a.opts, csr, x509cr) {
			glog.Infof("validator %q: denied CSR %q", r.name, csr.Name)
			csrApprovalStatus.WithLabelValues("deny", r.authFlowLabel).Inc()
			csrApprovalLatency.WithLabelValues("deny", r.authFlowLabel).Observe(time.Since(start).Seconds())
			return a.updateCSR(csr, false, r.denyMsg)
		}
		glog.Infof("CSR %q validation passed", csr.Name)

		approved, err := a.authorizeSAR(csr, r.permission)
		if err != nil {
			csrApprovalStatus.WithLabelValues("sar_error", r.authFlowLabel).Inc()
			csrApprovalLatency.WithLabelValues("sar_error", r.authFlowLabel).Observe(time.Since(start).Seconds())
			return err
		}
		if approved {
			glog.Infof("validator %q: SubjectAccessReview approved for CSR %q", r.name, csr.Name)
			csrApprovalStatus.WithLabelValues("approve", r.authFlowLabel).Inc()
			csrApprovalLatency.WithLabelValues("approve", r.authFlowLabel).Observe(time.Since(start).Seconds())
			return a.updateCSR(csr, true, r.approveMsg)
		} else {
			glog.Warningf("validator %q: SubjectAccessReview denied for CSR %q", r.name, csr.Name)
		}
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

type csrCheckFunc func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool

type csrValidator struct {
	name          string
	authFlowLabel string
	approveMsg    string
	denyMsg       string

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
		name:          "kubelet client certificate with TPM attestation and SubjectAccessReview",
		authFlowLabel: "kubelet_client_tpm",
		recognize:     isNodeClientCertWithAttestation,
		validate:      validateTPMAttestation,
		permission:    authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "nodeclient"},
		approveMsg:    "Auto approving kubelet client certificate with TPM attestation after SubjectAccessReview.",
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

func validateNodeServerCert(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	client := oauth2.NewClient(context.Background(), opts.TokenSource)

	if err := validateNodeServerCertInner(client, opts, csr, x509cr); err != nil {
		glog.Errorf("validating CSR %q: %v", csr.Name, err)
		return false
	}
	return true
}

// Only check that IPs in SAN match an existing VM in the project.
// Username was already checked against CN, so this CSR is coming from
// authenticated kubelet.
func validateNodeServerCertInner(client *http.Client, opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) error {
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
	for _, z := range opts.Zones {
		inst, err := srv.Get(opts.ProjectID, z, instanceName).Do()
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

func validateTPMAttestation(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	blocks, err := parsePEMBlocks(csr.Spec.Request)
	if err != nil {
		glog.Errorf("Parsing csr.Spec.Request: %v", err)
		return false
	}
	attestDataRaw := blocks["ATTESTATION DATA"].Bytes
	attestSig := blocks["ATTESTATION SIGNATURE"].Bytes
	attestCert := blocks["ATTESTATION CERTIFICATE"].Bytes

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

	nodeID, err := nodeidentity.FromAIKCert(aikCert)
	if err != nil {
		glog.Errorf("Failed extracting VM identity from EK certificate: %v", err)
		return false
	}
	hostname := strings.TrimPrefix("system:node:", x509cr.Subject.CommonName)
	if nodeID.Name != hostname {
		glog.Errorf("VM name in ATTESTATION CERTIFICATE (%q) doesn't match CommonName in x509 CSR (%q)", nodeID.Name, x509cr.Subject.CommonName)
		return false
	}
	if fmt.Sprint(nodeID.ProjectName) != opts.ProjectID {
		glog.Errorf("Received CSR for a different project Name (%d)", nodeID.ProjectName)
		return false
	}

	client := oauth2.NewClient(context.Background(), opts.TokenSource)
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
		ok, err := clusterHasInstance(client, opts.ProjectID, inst.Zone, opts.ClusterName, inst.Id)
		if err != nil {
			glog.Errorf("Checking VM membership in cluster: %v", err)
			return false
		}
		if !ok {
			glog.Errorf("VM %q doesn't belong to cluster %q", inst.Name, opts.ClusterName)
			return false
		}
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
