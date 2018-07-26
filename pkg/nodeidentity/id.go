// Package nodeidentity contains types and helper functions for GKE Nodes.
package nodeidentity

import (
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"strconv"

	"cloud.google.com/go/compute/metadata"
)

var cloudComputeInstanceIdentifierOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 1, 21}

// Identity uniquely identifies a GCE VM.
type Identity struct {
	Zone        string `json:"zone"`
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	ProjectID   uint64 `json:"project_id"`
	ProjectName string `json:"project_name"`
}

// FromMetadata builds VM Identity from GCE Metadata using default client.
func FromMetadata() (Identity, error) {
	var id Identity
	var err error

	projID, err := metadata.NumericProjectID()
	if err != nil {
		return id, err
	}
	id.ProjectID, err = strconv.ParseUint(projID, 10, 64)
	if err != nil {
		return id, err
	}
	id.ProjectName, err = metadata.ProjectID()
	if err != nil {
		return id, err
	}
	id.Zone, err = metadata.Zone()
	if err != nil {
		return id, err
	}
	instID, err := metadata.InstanceID()
	if err != nil {
		return id, err
	}
	id.ID, err = strconv.ParseUint(instID, 10, 64)
	if err != nil {
		return id, err
	}
	id.Name, err = metadata.InstanceName()
	if err != nil {
		return id, err
	}

	return id, nil
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

// FromAIKCert extracts VM Identity from cloudComputeInstanceIdentifier
// extension in cert.
func FromAIKCert(cert *x509.Certificate) (Identity, error) {
	var id asn1Identity
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(cloudComputeInstanceIdentifierOID) {
			continue
		}
		_, err := asn1.Unmarshal(ext.Value, &id)
		if err != nil {
			return Identity{}, err
		}
		return Identity{
			Zone:        id.Zone,
			ID:          uint64(id.ID),
			Name:        id.Name,
			ProjectID:   uint64(id.ProjectID),
			ProjectName: id.ProjectName,
		}, nil
	}
	return Identity{}, fmt.Errorf("certificate does not have cloudComputeInstanceIdentifier extension (OID %s)", cloudComputeInstanceIdentifierOID)
}
