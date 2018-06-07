package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	certificates "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"k8s.io/client-go/util/cert"
)

const (
	certFileName = "kubelet-client.crt"
	keyFileName  = "kubelet-client.key"

	rotationThreshold = 24 * time.Hour
)

func getKeyCert() ([]byte, []byte, error) {
	if key, cert, ok := getExistingKeyCert(*cacheDir); ok {
		glog.Info("re-using cached key and certificate")
		return key, cert, nil
	}

	glog.Info("generating new private key")
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: cert.ECPrivateKeyBlockType, Bytes: keyBytes})

	glog.Info("requesting new certificate")
	bootstrapConfig, err := loadRESTClientConfig(*bootstrapPath)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load bootstrap kubeconfig: %v", err)
	}
	bootstrapClient, err := certificates.NewForConfig(bootstrapConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create certificates signing request client: %v", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to determine hostnamename: %v", err)
	}

	certPEM, err := requestCertificate(bootstrapClient.CertificateSigningRequests(), keyPEM, hostname)
	if err != nil {
		return nil, nil, err
	}
	glog.Info("CSR approved, received certificate")

	if err := writeKeyCert(*cacheDir, keyPEM, certPEM); err != nil {
		return nil, nil, err
	}
	return keyPEM, certPEM, nil
}

func getExistingKeyCert(dir string) ([]byte, []byte, bool) {
	key, err := ioutil.ReadFile(filepath.Join(dir, keyFileName))
	if err != nil {
		return nil, nil, false
	}
	cert, err := ioutil.ReadFile(filepath.Join(dir, certFileName))
	if err != nil {
		return nil, nil, false
	}
	// Check cert expiration.
	certRaw, _ := pem.Decode(cert)
	if certRaw != nil {
		glog.Error("failed parsing existing cert")
		return nil, nil, false
	}
	parsedCert, err := x509.ParseCertificate(certRaw.Bytes)
	if err != nil {
		glog.Errorf("failed parsing existing cert: %v", err)
		return nil, nil, false
	}
	if diff := time.Now().Sub(parsedCert.NotAfter); diff < rotationThreshold {
		if diff < 0 {
			glog.Warningf("existing cert expired %v ago, requesting new one", -diff)
		} else {
			glog.Infof("existing cert will expire in %v, requesting new one", diff)
		}
		return nil, nil, false
	}
	return key, cert, true
}

func writeKeyCert(dir string, key, cert []byte) error {
	if err := ioutil.WriteFile(filepath.Join(dir, keyFileName), key, os.FileMode(0600)); err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(dir, certFileName), cert, os.FileMode(0644))
}
