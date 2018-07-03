package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"reflect"

	"github.com/golang/glog"
	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"
	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
	"k8s.io/cloud-provider-gcp/pkg/tpmattest"
)

const (
	// Documented constant NVRAM addresses for AIK template and certificate
	// inside the TPM.
	aikCertIndex     = 0x01c10000
	aikTemplateIndex = 0x01c10001
)

// TPM 2.0 specification can be found at
// https://trustedcomputinggroup.org/resource/tpm-library-specification/
//
// Most relevant are "Part 1: Architecture" and  "Part 3: Commands".

type tpmDevice interface {
	createPrimaryRawTemplate([]byte) (tpmutil.Handle, crypto.PublicKey, error)
	certify(tpmutil.Handle, tpmutil.Handle) ([]byte, []byte, error)
	nvRead(tpmutil.Handle) ([]byte, error)
	loadExternal(tpm2.Public, tpm2.Private) (tpmutil.Handle, error)
	flush(tpmutil.Handle)
	close() error
}

type realTPM struct {
	rwc io.ReadWriteCloser
}

func openTPM(path string) (*realTPM, error) {
	rw, err := tpm2.OpenTPM(path)
	if err != nil {
		return nil, fmt.Errorf("tpm2.OpenTPM(%q): %v", path, err)
	}
	return &realTPM{rw}, nil
}

func (t *realTPM) createPrimaryRawTemplate(pub []byte) (tpmutil.Handle, crypto.PublicKey, error) {
	return tpm2.CreatePrimaryRawTemplate(t.rwc, tpm2.HandleEndorsement, tpm2.PCRSelection{}, "", "", pub)
}
func (t *realTPM) certify(kh, aikh tpmutil.Handle) ([]byte, []byte, error) {
	return tpm2.Certify(t.rwc, "", "", kh, aikh, nil)
}
func (t *realTPM) nvRead(h tpmutil.Handle) ([]byte, error) {
	return tpm2.NVRead(t.rwc, h)
}
func (t *realTPM) loadExternal(pub tpm2.Public, priv tpm2.Private) (tpmutil.Handle, error) {
	kh, _, err := tpm2.LoadExternal(t.rwc, pub, priv, tpm2.HandleNull)
	return kh, err
}
func (t *realTPM) flush(h tpmutil.Handle) {
	if err := tpm2.FlushContext(t.rwc, h); err != nil {
		glog.Errorf("tpm2.Flush(0x%x): %v", h, err)
	}
}
func (t *realTPM) close() error { return t.rwc.Close() }

// We don't have GCE metadata during tests, allow override.
var newNodeIdentity = nodeidentity.FromMetadata

// tpmAttest generates an attestation signature for privateKey using AIK in
// TPM. Returned bytes are concatenated PEM blocks of the signature,
// attestation data and AIK certificate.
//
// High-level flow (TPM commands in parens):
// - load AIK from template in NVRAM (TPM2_NV_ReadPublic, TPM2_NV_Read,
//   TPM2_CreatePrimary)
// - load privateKey into the TPM (TPM2_LoadExternal)
// - certify (sign) privateKey with AIK (TPM2_Certify)
// - read AIK certificate from NVRAM (TPM2_NV_ReadPubluc, TPM2_NV_Read)
func tpmAttest(dev tpmDevice, privateKey crypto.PrivateKey) ([]byte, error) {
	aikh, aikPub, err := loadPrimaryKey(dev)
	if err != nil {
		return nil, fmt.Errorf("loadPrimaryKey: %v", err)
	}
	defer dev.flush(aikh)
	glog.Info("loaded AIK")

	kh, err := loadTLSKey(dev, privateKey)
	if err != nil {
		return nil, fmt.Errorf("loadTLSKey: %v", err)
	}
	defer dev.flush(kh)
	glog.Info("loaded TLS key")

	attest, sig, err := dev.certify(kh, aikh)
	if err != nil {
		return nil, fmt.Errorf("certify failed: %v", err)
	}
	glog.Info("TLS key certified by AIK")

	// Sanity-check the signature.
	attestHash := sha256.Sum256(attest)
	if err := rsa.VerifyPKCS1v15(aikPub.(*rsa.PublicKey), crypto.SHA256, attestHash[:], sig); err != nil {
		return nil, fmt.Errorf("Signature verification failed: %v", err)
	}
	glog.Info("certification signature verified with AIK public key")

	aikCertRaw, aikCert, err := readAIKCert(dev, aikh, aikPub)
	if err != nil {
		return nil, fmt.Errorf("reading AIK cert: %v", err)
	}
	glog.Info("AIK cert loaded")

	// Sanity-check that AIK cert matches AIK.
	aikCertPub := aikCert.PublicKey.(*rsa.PublicKey)
	if !reflect.DeepEqual(aikPub, aikCertPub) {
		return nil, fmt.Errorf("AIK public key doesn't match certificate public key")
	}
	if err := rsa.VerifyPKCS1v15(aikCertPub, crypto.SHA256, attestHash[:], sig); err != nil {
		return nil, fmt.Errorf("verifying certification signature with AIK cert: %v", err)
	}

	id, err := newNodeIdentity()
	if err != nil {
		return nil, fmt.Errorf("fetching VM identity from GCE metadata: %v", err)
	}
	idRaw, err := json.Marshal(id)
	if err != nil {
		return nil, fmt.Errorf("marshaling VM identity: %v", err)
	}

	return bytes.Join([][]byte{
		pem.EncodeToMemory(&pem.Block{
			Type:  "ATTESTATION CERTIFICATE",
			Bytes: aikCertRaw,
		}),
		pem.EncodeToMemory(&pem.Block{
			Type:  "ATTESTATION DATA",
			Bytes: attest,
		}),
		pem.EncodeToMemory(&pem.Block{
			Type:  "ATTESTATION SIGNATURE",
			Bytes: sig,
		}),
		pem.EncodeToMemory(&pem.Block{
			Type:  "VM IDENTITY",
			Bytes: idRaw,
		}),
	}, nil), nil
}

func loadPrimaryKey(dev tpmDevice) (tpmutil.Handle, crypto.PublicKey, error) {
	aikTemplate, err := dev.nvRead(aikTemplateIndex)
	if err != nil {
		return 0, nil, fmt.Errorf("tpm2.NVRead(AIK template): %v", err)
	}
	aikh, aikPub, err := dev.createPrimaryRawTemplate(aikTemplate)
	if err != nil {
		return 0, nil, fmt.Errorf("tpm2.CreatePrimary: %v", err)
	}
	return aikh, aikPub, nil
}

func readAIKCert(dev tpmDevice, aikh tpmutil.Handle, aikPub crypto.PublicKey) ([]byte, *x509.Certificate, error) {
	aikCert, err := dev.nvRead(tpmutil.Handle(aikCertIndex))
	if err != nil {
		return nil, nil, fmt.Errorf("tpm2.NVRead(AIK cert): %v", err)
	}

	crt, err := x509.ParseCertificate(aikCert)
	if err != nil {
		return aikCert, nil, fmt.Errorf("parsing AIK cert: %v", err)
	}
	return aikCert, crt, nil
}

func loadTLSKey(dev tpmDevice, privateKey crypto.PrivateKey) (tpmutil.Handle, error) {
	pk, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return 0, fmt.Errorf("only EC keys are supported, got %T", privateKey)
	}
	pub, err := tpmattest.MakePublic(pk.Public())
	if err != nil {
		return 0, fmt.Errorf("failed to build TPMT_PUBLIC struct: %v", err)
	}
	private := tpm2.Private{
		Type:      tpm2.AlgECC,
		Sensitive: pk.D.Bytes(),
	}
	kh, err := dev.loadExternal(pub, private)
	if err != nil {
		return 0, fmt.Errorf("loadExternal: %v", err)
	}
	return kh, nil
}
