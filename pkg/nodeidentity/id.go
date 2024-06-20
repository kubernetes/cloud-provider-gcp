// Package nodeidentity contains types and helper functions for GKE Nodes.
package nodeidentity

import (
	"encoding/asn1"
)

// CloudComputeInstanceIdentifierOID is an x509 Extension OID for VM Identity info.
var CloudComputeInstanceIdentifierOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 1, 21}

// Identity uniquely identifies a GCE VM.
type Identity struct {
	Zone        string `json:"zone"`
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	ProjectID   uint64 `json:"project_id"`
	ProjectName string `json:"project_name"`
}

// We need this separate struct because encoding/asn1 doesn't understand
// uint64.
type asn1Identity struct {
	Zone        string
	ID          int64
	Name        string
	ProjectID   int64
	ProjectName string
}

// ToASN1 serializes Identity to ASN1 format used in
// CloudComputeInstanceIdentifiedOID x509 extension.
func (id Identity) ToASN1() ([]byte, error) {
	return asn1.Marshal(asn1Identity{
		Zone:        id.Zone,
		ID:          int64(id.ID),
		Name:        id.Name,
		ProjectID:   int64(id.ProjectID),
		ProjectName: id.ProjectName,
	})
}
