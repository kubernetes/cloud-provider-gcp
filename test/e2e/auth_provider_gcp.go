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

package e2e

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/provider"
	credentialproviderapi "k8s.io/kubelet/pkg/apis/credentialprovider/v1"
)

var _ = Describe("[cloud-provider-gcp-e2e] auth-provider-gcp Workload Identity Federation", func() {
	const (
		image          = "us-central1-docker.pkg.dev/test-project/test-repository/test-image:latest"
		ksaToken       = "header.payload.signature"
		federatedToken = "federated-access-token"
		stsAudience    = "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/test-pool/providers/test-provider"
	)

	It("exchanges a KSA token using the configured STS audience", func(ctx context.Context) {
		requests := make(chan stsExchangeRequest, 1)
		transport := newSTSTransport(ctx, requests, federatedToken, nil)
		request := credentialproviderapi.CredentialProviderRequest{
			Image:               image,
			ServiceAccountToken: ksaToken,
		}
		registryProvider := provider.MakeRegistryProvider(transport, request, "", "", stsAudience)

		Expect(registryProvider.Enabled()).To(BeTrue(), "a configured STS audience must enable the provider outside GCE")
		response, err := provider.GetResponse(request, registryProvider)
		Expect(err).NotTo(HaveOccurred())

		var stsRequest stsExchangeRequest
		Eventually(requests).WithContext(ctx).Should(Receive(&stsRequest))
		Expect(stsRequest.Audience).To(Equal(stsAudience))
		Expect(stsRequest.SubjectToken).To(Equal(ksaToken))

		registry := "us-central1-docker.pkg.dev"
		Expect(response.Auth).To(HaveKey(registry))
		Expect(response.Auth[registry].Username).To(Equal("_token"))
		Expect(response.Auth[registry].Password).To(Equal(federatedToken))
	})

	It("fails fast without a KSA token", func(ctx context.Context) {
		var calls atomic.Int32
		transport := newSTSTransport(ctx, nil, federatedToken, &calls)
		request := credentialproviderapi.CredentialProviderRequest{Image: image}
		registryProvider := provider.MakeRegistryProvider(transport, request, "", "", stsAudience)

		response, err := provider.GetResponse(request, registryProvider)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Auth).To(BeEmpty())
		Expect(calls.Load()).To(BeZero(), "STS must not be called without a KSA token")
	})
})

type stsExchangeRequest struct {
	Audience     string `json:"audience"`
	SubjectToken string `json:"subject_token"`
}

func newSTSTransport(ctx context.Context, requests chan<- stsExchangeRequest, accessToken string, calls *atomic.Int32) *http.Transport {
	certificate := newSTSCertificate()
	return utilnet.SetTransportDefaults(&http.Transport{
		DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
			if address != "sts.googleapis.com:443" {
				return nil, fmt.Errorf("unexpected outbound connection to %q", address)
			}
			if calls != nil {
				calls.Add(1)
			}
			client, server := net.Pipe()
			go serveSTSConnection(ctx, server, certificate, requests, accessToken)
			return client, nil
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // The in-memory endpoint uses a per-test certificate.
			MinVersion:         tls.VersionTLS12,
		},
		ForceAttemptHTTP2: false,
	})
}

func serveSTSConnection(ctx context.Context, connection net.Conn, certificate tls.Certificate, requests chan<- stsExchangeRequest, accessToken string) {
	defer connection.Close()
	tlsConnection := tls.Server(connection, &tls.Config{
		Certificates: []tls.Certificate{certificate},
		NextProtos:   []string{"http/1.1"},
		MinVersion:   tls.VersionTLS12,
	})
	if err := tlsConnection.HandshakeContext(ctx); err != nil {
		return
	}
	request, err := http.ReadRequest(bufio.NewReader(tlsConnection))
	if err != nil {
		return
	}
	defer request.Body.Close()
	if request.Method != http.MethodPost || request.URL.Path != "/v1/token" {
		return
	}

	var payload stsExchangeRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		return
	}
	if requests != nil {
		select {
		case requests <- payload:
		case <-ctx.Done():
			return
		}
	}

	responseBody := fmt.Sprintf(`{"access_token":%q,"expires_in":3600,"token_type":"Bearer"}`, accessToken)
	response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(responseBody), responseBody)
	_, _ = io.WriteString(tlsConnection, response)
}

func newSTSCertificate() tls.Certificate {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sts.googleapis.com"},
		DNSNames:     []string{"sts.googleapis.com"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	Expect(err).NotTo(HaveOccurred())
	return certificate
}
