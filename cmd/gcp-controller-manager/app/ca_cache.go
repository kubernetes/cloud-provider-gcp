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
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// This should (almost) never change. If root key is compromised, rotation
	// will be a long and painful effort.
	rootCertURL = "https://pki.goog/cloud_integrity/tpm_ek_root_1.crt"
	// Intermediate CA URL looks like
	// http://pki.goog/cloud_integrity/tpm_ek_intermediate_h1_2018.crt.
	intermediateCAPrefix = "http://pki.goog/cloud_integrity/tpm_ek_intermediate_"

	crlCacheDuration = 10 * time.Minute
)

type caCache struct {
	rootCertURL string
	interPrefix string

	mu    sync.RWMutex
	certs map[string]*x509.Certificate
	crls  map[string]*cachedCRL
}

type cachedCRL struct {
	crl       *pkix.CertificateList
	fetchedAt time.Time
}

// verify checks that cert is signed by the CA and has not been revoked.
func (c *caCache) verify(cert *x509.Certificate) error {
	// Check leaf cert issuers against a known intermediate CA prefix.
	var interCertURL string
	for _, url := range cert.IssuingCertificateURL {
		if strings.HasPrefix(url, c.interPrefix) {
			interCertURL = url
			break
		}
	}
	if interCertURL == "" {
		return fmt.Errorf("none of the CAs %q in leaf cert start with %q", cert.IssuingCertificateURL, c.interPrefix)
	}
	// Get intermediate and root CA certs.
	interCert, err := c.getCert(interCertURL)
	if err != nil {
		return err
	}
	rootCert, err := c.getCert(c.rootCertURL)
	if err != nil {
		return err
	}
	// Verify leaf cert chain up to root CA.
	opts := x509.VerifyOptions{
		Roots:         x509.NewCertPool(),
		Intermediates: x509.NewCertPool(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}
	opts.Roots.AddCert(rootCert)
	opts.Intermediates.AddCert(interCert)
	if _, err := cert.Verify(opts); err != nil {
		return err
	}

	// Check intermediate and root CRLs for revoked certs.
	if err := c.checkCRLs(interCert, cert); err != nil {
		return fmt.Errorf("checking intermediate CA CRL: %v", err)
	}
	if err := c.checkCRLs(rootCert, interCert); err != nil {
		return fmt.Errorf("checking root CA CRL: %v", err)
	}
	return nil
}

// getCert returns cached certificate for given URL or fetches it.
func (c *caCache) getCert(url string) (*x509.Certificate, error) {
	// First, check with a read-lock to avoid global locking.
	c.mu.RLock()
	crt, ok := c.certs[url]
	c.mu.RUnlock()
	if ok {
		return crt, nil
	}

	// Not in cache. Grab the global write lock.
	c.mu.Lock()
	defer c.mu.Unlock()
	// Check again to make sure another goroutine didn't fetch this cert before
	// we got the lock.
	crt, ok = c.certs[url]
	if ok {
		return crt, nil
	}

	// Fetch and cache the cert.
	crt, err := fetchCert(url)
	if err != nil {
		return nil, err
	}
	c.certs[url] = crt
	return crt, nil
}

// checkCRLs checks all CRLDistributionPoints in child, making sure child isn't
// revoked. If child is revoked or CRL isn't signed by parent, return an error.
func (c *caCache) checkCRLs(parent, child *x509.Certificate) error {
	if len(child.CRLDistributionPoints) == 0 {
		return fmt.Errorf("CRLDistributionPoints empty in child certificate")
	}
	for _, url := range child.CRLDistributionPoints {
		crl, err := c.getCRL(url, parent)
		if err != nil {
			return err
		}
		for _, rc := range crl.TBSCertList.RevokedCertificates {
			if rc.SerialNumber.Cmp(child.SerialNumber) == 0 {
				return fmt.Errorf("certificate serial number %s was revoked", child.SerialNumber)
			}
		}
	}
	return nil
}

// getCRL returns cached CRL for given URL or fetches it. CRL must be signed by
// cert. If cached CRL is old, a fresh copy is fetched.
func (c *caCache) getCRL(url string, cert *x509.Certificate) (*pkix.CertificateList, error) {
	// See comments in getCert for explanation of locking pattern.
	//
	// getCRL also ignores cached CRLs if they are older than the threshold.
	c.mu.RLock()
	crl, ok := c.crls[url]
	c.mu.RUnlock()
	if ok && time.Since(crl.fetchedAt) < crlCacheDuration {
		return crl.crl, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	crl, ok = c.crls[url]
	if ok && time.Since(crl.fetchedAt) < crlCacheDuration {
		return crl.crl, nil
	}

	crlRaw, err := fetchCRL(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %q: %v", url, err)
	}
	if err := cert.CheckCRLSignature(crlRaw); err != nil {
		return nil, fmt.Errorf("verifying CRL signature for %q: %v", url, err)
	}
	c.crls[url] = &cachedCRL{crl: crlRaw, fetchedAt: time.Now()}
	return crlRaw, nil
}

func fetchCert(url string) (*x509.Certificate, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(raw)
}

func fetchCRL(url string) (*pkix.CertificateList, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return x509.ParseDERCRL(raw)
}
