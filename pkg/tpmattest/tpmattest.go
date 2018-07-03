// Package tpmattest contains TPM attestation logic shared by
// gcp-controller-manager and gke-exec-auth-plugin.
package tpmattest

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"

	"github.com/google/go-tpm/tpm2"
)

var toTPMCurves = map[string]tpm2.EllipticCurve{
	elliptic.P224().Params().Name: tpm2.CurveNISTP224,
	elliptic.P256().Params().Name: tpm2.CurveNISTP256,
	elliptic.P384().Params().Name: tpm2.CurveNISTP384,
	elliptic.P521().Params().Name: tpm2.CurveNISTP521,
}

// MakePublic creates a tpm2.Public struct for pub. This struct is a TPM
// representation of the public key and is attested by Endorsement Key.
//
// Only *ecdsa.PublicKey is allowed.
func MakePublic(pub crypto.PublicKey) (tpm2.Public, error) {
	ecKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return tpm2.Public{}, fmt.Errorf("public key in CSR is %T, only ECDSA keys supported", pub)
	}

	curveName := ecKey.Curve.Params().Name
	curve, ok := toTPMCurves[curveName]
	if !ok {
		return tpm2.Public{}, fmt.Errorf("public key EC curve %q not supported", curveName)
	}
	return tpm2.Public{
		Type:    tpm2.AlgECC,
		NameAlg: tpm2.AlgSHA256,
		// Allow access for TPM user with password auth. FlagSign is required
		// to load the key (either Sign or Decrypt).
		Attributes: tpm2.FlagSign | tpm2.FlagUserWithAuth,
		ECCParameters: &tpm2.ECCParams{
			CurveID: curve,
			Sign: &tpm2.SigScheme{
				Alg:  tpm2.AlgECDSA,
				Hash: tpm2.AlgSHA256,
			},
			Point: tpm2.ECPoint{
				X: ecKey.X,
				Y: ecKey.Y,
			},
		},
	}, nil
}
