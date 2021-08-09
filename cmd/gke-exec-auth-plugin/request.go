package main

import (
	"context"
	"crypto/sha512"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"

	apicertificates "k8s.io/api/certificates/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/certificate/csr"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog/v2"
)

const (
	kubeEnvMetadata   = "instance/attributes/kube-env"
	kubeEnvCert       = "TPM_BOOTSTRAP_CERT: "
	kubeEnvKey        = "TPM_BOOTSTRAP_KEY: "
	kubeEnvMaster     = "KUBERNETES_MASTER_NAME: "
	kubeEnvCAFilePath = "CA_FILE_PATH: "

	// Used if CA_FILE_PATH not specified in kube-env.
	defaultCAFilePath = "/etc/srv/kubernetes/pki/ca-certificates.crt"
)

func requestCertificate(privateKey []byte) ([]byte, error) {
	kubeEnv, err := metadata.Get(kubeEnvMetadata)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch kube-env: %v", err)
	}
	conf, err := kubeEnvToConfig(kubeEnv)
	if err != nil {
		return nil, fmt.Errorf("unable to build REST config from kube-env: %v", err)
	}
	client, err := clientset.NewForConfig(conf)
	if err != nil {
		return nil, fmt.Errorf("unable to create certificates signing request client: %v", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to determine hostnamename: %v", err)
	}

	return processCSR(client, privateKey, hostname)
}

func kubeEnvToConfig(kubeEnv string) (*rest.Config, error) {
	caFilePath := defaultCAFilePath
	// Scan each line looking at prefixes, extract the ones we care about.
	lines := strings.Split(kubeEnv, "\n")
	var key, cert, master string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, kubeEnvCert):
			cert = strings.TrimPrefix(l, kubeEnvCert)
			certBytes, err := base64.StdEncoding.DecodeString(cert)
			if err != nil {
				return nil, fmt.Errorf("decoding %q in kube-env: %v", kubeEnvCert, err)
			}
			cert = string(certBytes)
		case strings.HasPrefix(l, kubeEnvKey):
			key = strings.TrimPrefix(l, kubeEnvKey)
			keyBytes, err := base64.StdEncoding.DecodeString(key)
			if err != nil {
				return nil, fmt.Errorf("decoding %q in kube-env: %v", kubeEnvKey, err)
			}
			key = string(keyBytes)
		case strings.HasPrefix(l, kubeEnvMaster):
			master = strings.TrimPrefix(l, kubeEnvMaster)
		case strings.HasPrefix(l, kubeEnvCAFilePath):
			caFilePath = strings.TrimPrefix(l, kubeEnvCAFilePath)
			klog.Infof("Using CA file path from kube-env: %q", caFilePath)
		}
	}
	if key == "" || cert == "" || master == "" {
		return nil, errors.New("kube-env doesn't have bootstrap credentials or master IP")
	}
	return &rest.Config{
		Host: "https://" + master,
		TLSClientConfig: rest.TLSClientConfig{
			CertData: []byte(cert),
			KeyData:  []byte(key),
			CAFile:   caFilePath,
		},
		Timeout: 5 * time.Minute,
	}, nil
}

// processCSR will create a certificate signing request for a node
// (Organization and CommonName for the CSR will be set as expected for node
// certificates) and send it to API server, then it will watch the object's
// status, once approved by API server, it will return the API server's issued
// certificate (pem-encoded). If there is any errors, or the watch timeouts, it
// will return an error.
func processCSR(client clientset.Interface, privateKeyData []byte, hostname string) ([]byte, error) {
	subject := &pkix.Name{
		Organization: []string{"system:nodes"},
		CommonName:   "system:node:" + hostname,
	}

	privateKey, err := keyutil.ParsePrivateKeyPEM(privateKeyData)
	if err != nil {
		return nil, fmt.Errorf("invalid private key for certificate request: %v", err)
	}
	csrData, err := cert.MakeCSR(privateKey, subject, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to generate certificate request: %v", err)
	}
	klog.Info("CSR generated")

	tpm, err := openTPM()
	if err != nil {
		return nil, fmt.Errorf("failed opening TPM device: %v", err)
	}
	defer tpm.close()
	attestData, err := tpmAttest(tpm, privateKey)
	if err != nil {
		return nil, fmt.Errorf("unable to add TPM attestation: %v", err)
	}
	csrData = append(csrData, attestData...)
	klog.Info("added TPM attestation")

	usages := []apicertificates.KeyUsage{
		apicertificates.UsageDigitalSignature,
		apicertificates.UsageKeyEncipherment,
		apicertificates.UsageClientAuth,
	}
	name := digestedName(privateKeyData, subject, usages)
	reqName, reqUID, err := csr.RequestCertificate(client, csrData, name, apicertificates.KubeAPIServerClientKubeletSignerName, nil, usages, privateKey)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.TODO(), 3600*time.Second)
	defer cancel()
	return csr.WaitForCertificate(ctx, client, reqName, reqUID)
}

// digestedName should include all the relevant pieces of the CSR we care about.
// We can't directly hash the serialized CSR because of random padding that we
// regenerate every loop and we include usages which are not contained in the
// CSR. This needs to be kept up to date as we add new fields to the node
// certificates and with ensureCompatible.
func digestedName(privateKeyData []byte, subject *pkix.Name, usages []apicertificates.KeyUsage) string {
	hash := sha512.New512_256()

	// Here we make sure two different inputs can't write the same stream
	// to the hash. This delimiter is not in the base64.URLEncoding
	// alphabet so there is no way to have spill over collisions. Without
	// it 'CN:foo,ORG:bar' hashes to the same value as 'CN:foob,ORG:ar'
	const delimiter = '|'
	encode := base64.RawURLEncoding.EncodeToString

	write := func(data []byte) {
		hash.Write([]byte(encode(data)))
		hash.Write([]byte{delimiter})
	}

	write(privateKeyData)
	write([]byte(subject.CommonName))
	for _, v := range subject.Organization {
		write([]byte(v))
	}
	for _, v := range usages {
		write([]byte(v))
	}

	return "node-csr-" + encode(hash.Sum(nil))
}
