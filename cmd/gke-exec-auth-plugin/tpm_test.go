package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"

	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
)

const primaryHandle tpmutil.Handle = 1

func init() {
	newNodeIdentity = func() (nodeidentity.Identity, error) {
		return nodeidentity.Identity{}, nil
	}
}

type fakeTPM struct {
	primary       *rsa.PrivateKey
	loaded        map[tpmutil.Handle]tpm2.Public
	nextHandle    tpmutil.Handle
	returnAIKCert bool
}

func newFakeTPM(t *testing.T) *fakeTPM {
	primary, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeTPM{
		primary:       primary,
		loaded:        make(map[tpmutil.Handle]tpm2.Public),
		nextHandle:    primaryHandle + 1,
		returnAIKCert: true,
	}
}

func (t *fakeTPM) createPrimaryRawTemplate(pub []byte) (tpmutil.Handle, crypto.PublicKey, error) {
	return primaryHandle, t.primary.Public(), nil
}
func (t *fakeTPM) certify(kh, aikh tpmutil.Handle) ([]byte, []byte, error) {
	pub, ok := t.loaded[kh]
	if !ok {
		return nil, nil, fmt.Errorf("handle %v not loaded", kh)
	}
	if aikh != primaryHandle {
		return nil, nil, errors.New("aik handle passed to certify isn't primaryHandle")
	}
	attest, err := json.Marshal(pub)
	if err != nil {
		return nil, nil, err
	}
	attestHash := sha256.Sum256(attest)
	sig, err := rsa.SignPKCS1v15(rand.Reader, t.primary, crypto.SHA256, attestHash[:])
	if err != nil {
		return nil, nil, err
	}
	return attest, sig, nil
}
func (t *fakeTPM) nvRead(h tpmutil.Handle) ([]byte, error) {
	switch h {
	case aikTemplateIndex:
		return nil, nil
	case aikCertIndex:
		if !t.returnAIKCert {
			return nil, fmt.Errorf("NV handle 0x%x not found", h)
		}
		template := &x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				Organization: []string{"Acme Co"},
			},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(24 * time.Hour),
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
		}

		return x509.CreateCertificate(rand.Reader, template, template, t.primary.Public(), t.primary)
	default:
		return nil, fmt.Errorf("NV handle 0x%x not found", h)
	}
}
func (t *fakeTPM) loadExternal(pub tpm2.Public, priv tpm2.Private) (tpmutil.Handle, error) {
	t.loaded[t.nextHandle] = pub
	h := t.nextHandle
	t.nextHandle++
	return h, nil
}
func (t *fakeTPM) flush(h tpmutil.Handle) { delete(t.loaded, h) }
func (t *fakeTPM) close() error           { return nil }

func TestTPMAttest(t *testing.T) {
	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	dev := newFakeTPM(t)
	run := func(t *testing.T, wantBlocks []string) {
		blocksRaw, err := tpmAttest(dev, pk)
		if err != nil {
			t.Fatal(err)
		}
		if len(dev.loaded) > 0 {
			t.Errorf("%d handles not flushed in TPM", len(dev.loaded))
		}

		for i, want := range wantBlocks {
			var b *pem.Block
			b, blocksRaw = pem.Decode(blocksRaw)
			if b == nil {
				t.Fatalf("missing PEM blocks %q in attestation data", wantBlocks[i:])
			}
			if b.Type != want {
				t.Errorf("got block %q, want %q", b.Type, want)
			}
		}
	}

	t.Run("with AIK cert", func(t *testing.T) {
		run(t, []string{"ATTESTATION DATA", "ATTESTATION SIGNATURE", "VM IDENTITY", "ATTESTATION CERTIFICATE"})
	})
	t.Run("without AIK cert", func(t *testing.T) {
		dev.returnAIKCert = false
		run(t, []string{"ATTESTATION DATA", "ATTESTATION SIGNATURE", "VM IDENTITY"})
	})
}
