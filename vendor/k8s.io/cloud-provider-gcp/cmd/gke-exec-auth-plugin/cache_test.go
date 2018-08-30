package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetKeyCert(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "cache_test")
	if err != nil {
		t.Fatal(err)
	}

	validKey, validCert := genFakeKeyCert(t, time.Now(), time.Now().Add(24*time.Hour))
	expiredKey, expiredCert := genFakeKeyCert(t, time.Now().Add(-24*time.Hour), time.Now().Add(-1*time.Hour))
	almostExpiredKey, almostExpiredCert := genFakeKeyCert(t, time.Now(), time.Now().Add(responseExpiry/2))
	oldKey, oldCert := genFakeKeyCert(t, time.Now().Add(-1*(rotationThreshold+time.Hour)), time.Now().Add(24*time.Hour))
	futureKey, futureCert := genFakeKeyCert(t, time.Now().Add(time.Hour), time.Now().Add(24*time.Hour))

	tests := []struct {
		desc        string
		writeFiles  map[string][]byte
		requestCert requestCertFn
		wantKey     []byte
		wantCert    []byte
		wantErr     bool
	}{
		{
			desc: "reuse existing key and cert",
			writeFiles: map[string][]byte{
				certFileName: validCert,
				keyFileName:  validKey,
			},
			wantKey:  validKey,
			wantCert: validCert,
		},
		{
			desc: "request new key and cert",
			requestCert: func(keyPEM []byte) ([]byte, error) {
				return validCert, nil
			},
			wantCert: validCert,
		},
		{
			desc: "invalid existing cert",
			writeFiles: map[string][]byte{
				certFileName: []byte("invalid"),
				keyFileName:  validKey,
			},
			requestCert: func(keyPEM []byte) ([]byte, error) {
				return validCert, nil
			},
			wantCert: validCert,
		},
		{
			desc: "invalid existing key",
			writeFiles: map[string][]byte{
				certFileName: validCert,
				keyFileName:  []byte("invalid"),
			},
			requestCert: func(keyPEM []byte) ([]byte, error) {
				return almostExpiredCert, nil
			},
			wantCert: almostExpiredCert,
		},
		{
			desc: "expired existing cert",
			writeFiles: map[string][]byte{
				certFileName: expiredCert,
				keyFileName:  expiredKey,
			},
			requestCert: func(keyPEM []byte) ([]byte, error) {
				return validCert, nil
			},
			wantCert: validCert,
		},
		{
			desc: "not yet valid existing cert",
			writeFiles: map[string][]byte{
				certFileName: futureCert,
				keyFileName:  futureKey,
			},
			requestCert: func(keyPEM []byte) ([]byte, error) {
				return validCert, nil
			},
			wantCert: validCert,
		},
		{
			desc: "existing cert expiring soon rotated",
			writeFiles: map[string][]byte{
				certFileName: almostExpiredCert,
				keyFileName:  almostExpiredKey,
			},
			requestCert: func(keyPEM []byte) ([]byte, error) {
				return validCert, nil
			},
			wantCert: validCert,
		},
		{
			desc: "existing cert past rotation threshold rotated",
			writeFiles: map[string][]byte{
				certFileName: oldCert,
				keyFileName:  oldKey,
			},
			requestCert: func(keyPEM []byte) ([]byte, error) {
				return validCert, nil
			},
			wantCert: validCert,
		},
		{
			desc: "request new cert failure",
			requestCert: func(keyPEM []byte) ([]byte, error) {
				return nil, errors.New("foo")
			},
			wantErr: true,
		},
		{
			desc: "reuse existing temp key, request new cert",
			writeFiles: map[string][]byte{
				tmpKeyFileName: validKey,
			},
			requestCert: func(keyPEM []byte) ([]byte, error) {
				if !bytes.Equal(keyPEM, validKey) {
					return nil, fmt.Errorf("requestCert with key %q, expected %q", keyPEM, validKey)
				}
				return validCert, nil
			},
			wantKey:  validKey,
			wantCert: validCert,
		},
		{
			desc: "request new cert failure, reuse existing key and cert",
			writeFiles: map[string][]byte{
				certFileName: oldCert,
				keyFileName:  oldKey,
			},
			requestCert: func(keyPEM []byte) ([]byte, error) {
				return nil, errors.New("foo")
			},
			wantKey:  oldKey,
			wantCert: oldCert,
		},
		{
			desc: "invalid existing temp key, request new cert",
			writeFiles: map[string][]byte{
				tmpKeyFileName: []byte("invalid"),
			},
			requestCert: func(keyPEM []byte) ([]byte, error) {
				if bytes.Equal(keyPEM, []byte("invalid")) {
					return nil, fmt.Errorf("requestCert with key %q, expected new key", keyPEM)
				}
				return validCert, nil
			},
			wantCert: validCert,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Wipe out temp directory for each run.
			if err := os.MkdirAll(dir, 0777); err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			for fname, data := range tt.writeFiles {
				if err := ioutil.WriteFile(filepath.Join(dir, fname), data, 0777); err != nil {
					t.Fatal(err)
				}
			}

			var requestCert requestCertFn
			var requestCertCalled bool
			if tt.requestCert == nil {
				requestCert = func(keyPEM []byte) ([]byte, error) {
					t.Error("requestCert is called, but test.requestCert was nil")
					return nil, errors.New("failed")
				}
			} else {
				requestCert = func(keyPEM []byte) ([]byte, error) {
					requestCertCalled = true
					return tt.requestCert(keyPEM)
				}
			}
			gotKey, gotCert, err := getKeyCert(dir, requestCert)
			switch {
			case err == nil && tt.wantErr:
				t.Fatal("error is nil, expected non-nil error")
			case err != nil && !tt.wantErr:
				t.Fatalf("error is %q, expected nil", err)
			case err != nil && tt.wantErr:
				return
			}

			if tt.requestCert != nil && !requestCertCalled {
				t.Error("requestCert wasn't called")
			}

			// If wantKey isn't set and tt.wantErr was false, we expect a new
			// key to be generated in getKeyCert.
			if tt.wantKey != nil {
				if !bytes.Equal(gotKey, tt.wantKey) {
					t.Errorf("got key:\n%q\nwant:\n%q", gotKey, tt.wantKey)
				}
			} else {
				if len(gotKey) == 0 {
					t.Errorf("got empty key, want non-empty")
				}
			}
			if !bytes.Equal(gotCert, tt.wantCert) {
				t.Errorf("got cert:\n%q\nwant:\n%q", gotCert, tt.wantCert)
			}

			// Check that key and cert got cached.
			diskKey, err := ioutil.ReadFile(filepath.Join(dir, keyFileName))
			if err != nil {
				t.Fatalf("reading cached key: %v", err)
			}
			if !bytes.Equal(diskKey, gotKey) {
				t.Errorf("got key on disk:\n%q\nwant:\n%q", diskKey, gotKey)
			}
			diskCert, err := ioutil.ReadFile(filepath.Join(dir, certFileName))
			if err != nil {
				t.Fatalf("reading cached cert: %v", err)
			}
			if !bytes.Equal(diskCert, gotCert) {
				t.Errorf("got cert on disk:\n%q\nwant:\n%q", diskCert, gotCert)
			}
		})
	}
}

func genFakeKeyCert(t *testing.T, validFrom, validTo time.Time) ([]byte, []byte) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore:             validFrom,
		NotAfter:              validTo,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	cert, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})

	return keyPEM, certPEM
}
