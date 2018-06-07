package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

const primaryHandle tpmutil.Handle = 1

type fakeTPM struct {
	primary    *rsa.PrivateKey
	loaded     map[tpmutil.Handle]tpm2.Public
	nextHandle tpmutil.Handle
}

func newFakeTPM(t *testing.T) *fakeTPM {
	primary, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeTPM{
		primary:    primary,
		loaded:     make(map[tpmutil.Handle]tpm2.Public),
		nextHandle: primaryHandle + 1,
	}
}

func (t *fakeTPM) createPrimary(pub tpm2.Public) (tpmutil.Handle, crypto.PublicKey, error) {
	return primaryHandle, t.primary.Public(), nil
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
	// TODO(awly): when aikCert/Template are available, add hardcoded values.
	return nil, errors.New("not supported")
}
func (t *fakeTPM) loadExternal(pub tpm2.Public, priv tpm2.Private) (tpmutil.Handle, error) {
	t.loaded[t.nextHandle] = pub
	h := t.nextHandle
	t.nextHandle++
	return h, nil
}
func (t *fakeTPM) privateKey(h tpmutil.Handle, pub crypto.PublicKey) crypto.PrivateKey {
	if h != primaryHandle {
		return nil
	}
	return t.primary
}
func (t *fakeTPM) flush(h tpmutil.Handle) { delete(t.loaded, h) }
func (t *fakeTPM) close() error           { return nil }

func TestTPMAttest(t *testing.T) {
	dev := newFakeTPM(t)
	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	blocksRaw, err := tpmAttest(dev, pk)
	if err != nil {
		t.Fatal(err)
	}
	if len(dev.loaded) > 0 {
		t.Errorf("%d handles not flushed in TPM", len(dev.loaded))
	}

	wantBlocks := []string{"ATTESTATION CERTIFICATE", "ATTESTATION DATA", "ATTESTATION SIGNATURE"}
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
