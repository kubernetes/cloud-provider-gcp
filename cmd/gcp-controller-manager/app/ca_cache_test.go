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
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
)

func TestCACacheVerify(t *testing.T) {
	ca, c, cleanup := initFakeCACache(t)
	defer cleanup()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		if err := c.verify(ca.validCert); err != nil {
			t.Errorf("verifying valid certificate: got %v, want nil", err)
		}
	})
	if err := c.verify(ca.validCert); err != nil {
		t.Errorf("verifying valid certificate: got %v, want nil", err)
	}
	for desc, invalidCert := range ca.invalidCerts {
		t.Run(desc, func(t *testing.T) {
			t.Parallel()
			if err := c.verify(invalidCert); err == nil {
				t.Errorf("verifying invalid certificate: got nil, want non-nil error")
			}
		})
	}
}

func initFakeCACache(t *testing.T) (*fakeCA, *caCache, func()) {
	var ca *fakeCA
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		switch path.Base(r.URL.Path) {
		case "root.crt":
			rw.Write(ca.rootCert)
		case "intermediate.crt":
			rw.Write(ca.intermediateCertRaw)
		case "self-signed-intermediate.crt":
			rw.Write(ca.selfSignedIntermediateCert)
		case "root.crl":
			rw.Write(ca.rootCRL)
		case "intermediate.crl":
			rw.Write(ca.intermediateCRL)
		case "self-signed-intermediate.crl":
			rw.Write(ca.selfSignedIntermediateCRL)
		default:
			http.Error(rw, "not found", http.StatusNotFound)
		}
	}))

	ca = initFakeCA(t, srv.URL)
	c := &caCache{
		rootCertURL: srv.URL + "/root.crt",
		interPrefix: srv.URL,
		certs:       make(map[string]*x509.Certificate),
		crls:        make(map[string]*cachedCRL),
	}
	return ca, c, srv.Close
}

type fakeCA struct {
	rootCert, intermediateCertRaw, selfSignedIntermediateCert []byte
	rootCRL, intermediateCRL, selfSignedIntermediateCRL       []byte
	intermediateCert                                          *x509.Certificate
	intermediateCertKey                                       *rsa.PrivateKey
	validCert                                                 *x509.Certificate
	validCertKey                                              *rsa.PrivateKey
	invalidCerts                                              map[string]*x509.Certificate
	srvURL                                                    string
}

func initFakeCA(t *testing.T, srvURL string) *fakeCA {
	t.Helper()
	ca := &fakeCA{
		invalidCerts: make(map[string]*x509.Certificate),
		srvURL:       srvURL,
	}

	rootTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
		BasicConstraintsValid: true,
	}
	rootCertDER, rootCert, rootKey := makeCert(t, rootTmpl, rootTmpl, nil)
	ca.rootCert = rootCertDER
	interTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
		BasicConstraintsValid: true,
		CRLDistributionPoints: []string{srvURL + "/root.crl"},
	}
	ca.intermediateCertRaw, ca.intermediateCert, ca.intermediateCertKey = makeCert(t, interTmpl, rootCert, rootKey)

	ca.regenerateValidCert(t, nodeidentity.Identity{"z0", 1, "i0", 2, "p0"})

	_, ca.invalidCerts["revoked"], _ = makeCert(t, &x509.Certificate{
		SerialNumber:          big.NewInt(4),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		IssuingCertificateURL: []string{srvURL + "/intermediate.crt"},
		CRLDistributionPoints: []string{srvURL + "/intermediate.crl"},
	}, ca.intermediateCert, ca.intermediateCertKey)
	_, ca.invalidCerts["no CRL"], _ = makeCert(t, &x509.Certificate{
		SerialNumber:          big.NewInt(4),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		IssuingCertificateURL: []string{srvURL + "/intermediate.crt"},
	}, ca.intermediateCert, ca.intermediateCertKey)
	selfSignedTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(99),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	_, ca.invalidCerts["self signed"], _ = makeCert(t, selfSignedTmpl, selfSignedTmpl, nil)
	_, ca.invalidCerts["wrong intermediate link"], _ = makeCert(t, &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		IssuingCertificateURL: []string{"http://example.com/wrong-intermediate.crt"},
		CRLDistributionPoints: []string{srvURL + "/intermediate.crl"},
	}, ca.intermediateCert, ca.intermediateCertKey)

	selfSignedInterTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
		BasicConstraintsValid: true,
		CRLDistributionPoints: []string{srvURL + "/root.crl"},
	}
	selfSignedInterCertDER, selfSignedInterCert, selfSignedInterKey := makeCert(t, selfSignedInterTmpl, selfSignedInterTmpl, nil)
	ca.selfSignedIntermediateCert = selfSignedInterCertDER
	wrongInterLeafTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		IssuingCertificateURL: []string{srvURL + "/self-signed-intermediate.crt"},
		CRLDistributionPoints: []string{srvURL + "/self-signed-intermediate.crl"},
	}
	_, ca.invalidCerts["self-signed intermediate"], _ = makeCert(t, wrongInterLeafTmpl, selfSignedInterCert, selfSignedInterKey)

	var err error
	ca.rootCRL, err = rootCert.CreateCRL(insecureRand, rootKey, nil, time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ca.intermediateCRL, err = ca.intermediateCert.CreateCRL(insecureRand, ca.intermediateCertKey, []pkix.RevokedCertificate{{
		SerialNumber:   big.NewInt(4),
		RevocationTime: time.Now(),
	}}, time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ca.selfSignedIntermediateCRL, err = selfSignedInterCert.CreateCRL(insecureRand, selfSignedInterKey, nil, time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}

	return ca
}

func (ca *fakeCA) regenerateValidCert(t *testing.T, id nodeidentity.Identity) {
	vmID, err := id.ToASN1()
	if err != nil {
		t.Fatal(err)
	}
	_, ca.validCert, ca.validCertKey = makeCert(t, &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		IssuingCertificateURL: []string{ca.srvURL + "/intermediate.crt"},
		CRLDistributionPoints: []string{ca.srvURL + "/intermediate.crl"},
		ExtraExtensions: []pkix.Extension{{
			Id:    nodeidentity.CloudComputeInstanceIdentifierOID,
			Value: vmID,
		}},
	}, ca.intermediateCert, ca.intermediateCertKey)
}

func makeCert(t *testing.T, tmpl, parent *x509.Certificate, parentKey crypto.PrivateKey) ([]byte, *x509.Certificate, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(insecureRand, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if parentKey == nil {
		parentKey = key
	}
	certDER, err := x509.CreateCertificate(insecureRand, tmpl, parent, key.Public(), parentKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatal(err)
	}

	return certDER, cert, key
}
