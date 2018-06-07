package main

import (
	"crypto/sha512"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang/glog"
	apicertificates "k8s.io/api/certificates/v1beta1"
	certificates "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/certificate/csr"
)

func loadRESTClientConfig(kubeconfig string) (*rest.Config, error) {
	// Load structured kubeconfig data from the given path.
	loader := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	loadedConfig, err := loader.Load()
	if err != nil {
		return nil, err
	}
	// Flatten the loaded data to a particular rest.Config based on the current context.
	return clientcmd.NewNonInteractiveClientConfig(
		*loadedConfig,
		loadedConfig.CurrentContext,
		&clientcmd.ConfigOverrides{},
		loader,
	).ClientConfig()
}

// requestCertificate will create a certificate signing request for a node
// (Organization and CommonName for the CSR will be set as expected for node
// certificates) and send it to API server, then it will watch the object's
// status, once approved by API server, it will return the API server's issued
// certificate (pem-encoded). If there is any errors, or the watch timeouts, it
// will return an error.
func requestCertificate(client certificates.CertificateSigningRequestInterface, privateKeyData []byte, hostname string) ([]byte, error) {
	subject := &pkix.Name{
		Organization: []string{"system:nodes"},
		CommonName:   "system:node:" + hostname,
	}

	privateKey, err := cert.ParsePrivateKeyPEM(privateKeyData)
	if err != nil {
		return nil, fmt.Errorf("invalid private key for certificate request: %v", err)
	}
	csrData, err := cert.MakeCSR(privateKey, subject, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to generate certificate request: %v", err)
	}
	glog.Info("CSR generated")
	tpm, err := openTPM(*tpmPath)
	if err != nil {
		return nil, err
	}
	defer tpm.close()
	attestData, err := tpmAttest(tpm, privateKey)
	if err != nil {
		return nil, fmt.Errorf("unable to add TPM attestation: %v", err)
	}
	csrData = append(csrData, attestData...)
	glog.Info("added TPM attestation")

	usages := []apicertificates.KeyUsage{
		apicertificates.UsageDigitalSignature,
		apicertificates.UsageKeyEncipherment,
		apicertificates.UsageClientAuth,
	}
	name := digestedName(privateKeyData, subject, usages)
	req, err := csr.RequestCertificate(client, csrData, name, usages, privateKey)
	if err != nil {
		return nil, err
	}
	return csr.WaitForCertificate(client, req, 3600*time.Second)
}

// This digest should include all the relevant pieces of the CSR we care about.
// We can't direcly hash the serialized CSR because of random padding that we
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
