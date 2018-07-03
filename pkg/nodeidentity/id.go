// Package nodeidentity contains types and helper functions for GKE Nodes.
package nodeidentity

import (
	"strconv"

	"cloud.google.com/go/compute/metadata"
)

// Identity uniquely identifies a GCE VM.
type Identity struct {
	ProjectID   uint64 `json:"project_id"`
	ProjectName string `json:"project_name"`
	Zone        string `json:"zone"`
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
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
