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
	"reflect"
	"time"

	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog/v2"
)

const (
	certFileName   = "kubelet-client.crt"
	keyFileName    = "kubelet-client.key"
	tmpKeyFileName = "kubelet-client.key.tmp"

	// Minimum age of existing certificate before triggering rotation.
	// Assuming no rotation errors, this is cert rotation period.
	rotationThreshold = 10 * 24 * time.Hour // 10 days
	// Caching duration for caller - will exec this plugin after this period.
	responseExpiry = time.Hour
	// validityLeeway is applied to NotBefore field of existing cert to account
	// for clock skew.
	validityLeeway = 5 * time.Minute
)

type requestCertFn func([]byte) ([]byte, error)

func getKeyCert(dir string, requestCert requestCertFn) ([]byte, []byte, error) {
	oldKey, oldCert, ok := getExistingKeyCert(dir)
	if ok {
		klog.Info("re-using cached key and certificate")
		return oldKey, oldCert, nil
	}

	newKey, newCert, err := getNewKeyCert(dir, requestCert)
	if err != nil {
		if len(oldKey) == 0 || len(oldCert) == 0 {
			return nil, nil, err
		}
		klog.Errorf("failed rotating client certificate: %v", err)
		klog.Info("using existing key/cert that are still valid")
		return oldKey, oldCert, nil
	}
	return newKey, newCert, nil
}

func getNewKeyCert(dir string, requestCert requestCertFn) ([]byte, []byte, error) {
	keyPEM, err := getTempKeyPEM(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("trying to get private key: %v", err)
	}

	klog.Info("requesting new certificate")
	certPEM, err := requestCert(keyPEM)
	if err != nil {
		return nil, nil, err
	}
	klog.Info("CSR approved, received certificate")

	if err := writeKeyCert(dir, keyPEM, certPEM); err != nil {
		return nil, nil, err
	}
	return keyPEM, certPEM, nil
}

func getTempKeyPEM(dir string) ([]byte, error) {
	keyPEM, err := ioutil.ReadFile(filepath.Join(dir, tmpKeyFileName))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("trying to read temp private key: %v", err)
	}
	if err == nil && validPEMKey(keyPEM, nil) {
		return keyPEM, nil
	}

	// Either temp key doesn't exist or it's invalid.
	klog.Info("generating new private key")
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: keyutil.ECPrivateKeyBlockType, Bytes: keyBytes})
	// Write private key into temporary file to reuse in case of failure.
	if err := ioutil.WriteFile(filepath.Join(dir, tmpKeyFileName), keyPEM, 0600); err != nil {
		return nil, fmt.Errorf("failed to store new private key to temporary file: %v", err)
	}
	return keyPEM, nil
}

// validPEMKey returns true if key contains a valid PEM-encoded private key. If
// cert is non-nil, it checks that key matches cert.
func validPEMKey(key []byte, cert *x509.Certificate) bool {
	if len(key) == 0 {
		return false
	}
	keyBlock, _ := pem.Decode(key)
	if keyBlock == nil {
		return false
	}
	pk, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return false
	}
	if cert == nil {
		return true
	}
	return reflect.DeepEqual(cert.PublicKey, pk.Public())
}

func getExistingKeyCert(dir string) ([]byte, []byte, bool) {
	key, err := ioutil.ReadFile(filepath.Join(dir, keyFileName))
	if err != nil {
		klog.Warningf("failed reading existing private key: %v", err)
		return nil, nil, false
	}
	cert, err := ioutil.ReadFile(filepath.Join(dir, certFileName))
	if err != nil {
		klog.Warningf("failed reading existing certificate: %v", err)
		return nil, nil, false
	}
	// Check cert expiration.
	certRaw, _ := pem.Decode(cert)
	if certRaw == nil {
		klog.Error("failed parsing existing cert")
		return nil, nil, false
	}
	parsedCert, err := x509.ParseCertificate(certRaw.Bytes)
	if err != nil {
		klog.Errorf("failed parsing existing cert: %v", err)
		return nil, nil, false
	}
	if !validPEMKey(key, parsedCert) {
		klog.Error("existing private key is invalid or doesn't match existing certificate")
		return nil, nil, false
	}
	age := time.Since(parsedCert.NotBefore)
	remaining := time.Until(parsedCert.NotAfter)
	// Note: case order matters. Always check outside of expiry bounds first
	// and put cases that return non-nil key/cert at the bottom.
	switch {
	case remaining < responseExpiry:
		klog.Infof("existing cert expired or will expire in <%v, requesting new one", responseExpiry)
		return nil, nil, false
	case age+validityLeeway < 0:
		klog.Warningf("existing cert not valid yet, requesting new one")
		return nil, nil, false
	case age < rotationThreshold:
		return key, cert, true
	default:
		// Existing key/cert can still be reused but try to rotate.
		klog.Infof("existing cert is %v old, requesting new one", age)
		return key, cert, false
	}
}

func writeKeyCert(dir string, key, cert []byte) error {
	if err := os.Rename(filepath.Join(dir, tmpKeyFileName), filepath.Join(dir, keyFileName)); err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(dir, certFileName), cert, os.FileMode(0644))
}
