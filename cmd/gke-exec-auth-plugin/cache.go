package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/util/cert"
)

const (
	certFileName = "kubelet-client.crt"
	keyFileName  = "kubelet-client.key"

	// Minimum age of existing certificate before triggering rotation.
	// Assuming no rotation errors, this is cert rotation period.
	rotationThreshold = 24 * time.Hour
	// Caching duration for caller - will exec this plugin after this period.
	responseExpiry = time.Hour
)

func getKeyCert() ([]byte, []byte, error) {
	oldKey, oldCert, ok := getExistingKeyCert(*cacheDir)
	if ok {
		glog.Info("re-using cached key and certificate")
		return oldKey, oldCert, nil
	}

	newKey, newCert, err := getNewKeyCert(*cacheDir)
	if err != nil {
		if len(oldKey) == 0 || len(oldCert) == 0 {
			return nil, nil, err
		}
		glog.Errorf("failed rotating client certificate: %v", err)
		glog.Info("using existing key/cert that are still valid")
		return oldKey, oldCert, nil
	}
	return newKey, newCert, nil
}

func getNewKeyCert(dir string) ([]byte, []byte, error) {
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
	certPEM, err := requestCertificate(keyPEM)
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
	age := time.Now().Sub(parsedCert.NotBefore)
	switch {
	case age < 0:
		glog.Warningf("existing cert not valid yet, requesting new one")
		return nil, nil, false
	case age < rotationThreshold:
		return key, cert, true
	case parsedCert.NotAfter.Sub(time.Now()) < responseExpiry:
		glog.Infof("existing cert expired or will expire in <%v, requesting new one", responseExpiry)
		return nil, nil, false
	default:
		// Existing key/cert can still be reused but try to rotate.
		glog.Infof("existing cert is %v old, requesting new one", age)
		return key, cert, false
	}
}

func writeKeyCert(dir string, key, cert []byte) error {
	if err := ioutil.WriteFile(filepath.Join(dir, keyFileName), key, os.FileMode(0600)); err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(dir, certFileName), cert, os.FileMode(0644))
}
