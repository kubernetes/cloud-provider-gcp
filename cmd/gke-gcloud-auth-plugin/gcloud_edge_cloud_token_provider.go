package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// gcloudEdgeCloudTokenProvider provides gcloud edge-cloud tokens.
type gcloudEdgeCloudTokenProvider struct {
	project                   string
	location                  string
	clusterName               string
	impersonateServiceAccount string
	getTokenRaw               func(project string, location string, clusterName string, impersonateServiceAccount string) ([]byte, error)
}

// gcloudEdgeCloudToken holds types unmarshaled from the edge cloud access token in json format
type gcloudEdgeCloudToken struct {
	AccessToken string    `json:"accessToken"`
	TokenExpiry time.Time `json:"expireTime"`
}

func (p *gcloudEdgeCloudTokenProvider) token() (string, *time.Time, error) {
	edgeCloudTokenBytes, err := p.getTokenRaw(p.project, p.location, p.clusterName, p.impersonateServiceAccount)
	if err != nil {
		return "", nil, err
	}

	var tok gcloudEdgeCloudToken
	if err := json.Unmarshal(edgeCloudTokenBytes, &tok); err != nil {
		return "", nil, fmt.Errorf("error parsing gcloud output: %w", err)
	}

	return tok.AccessToken, &tok.TokenExpiry, nil
}

func (p *gcloudEdgeCloudTokenProvider) useCache() bool { return true }

func getGcloudEdgeCloudTokenRaw(project string, location string, clusterName string, impersonateServiceAccount string) ([]byte, error) {
	args := []string{"edge-cloud", "container", "clusters", "print-access-token", clusterName, fmt.Sprintf("--project=%s", project), fmt.Sprintf("--location=%s", location), "--format=json"}
	if impersonateServiceAccount != "" {
		args = append(args, fmt.Sprintf("--impersonate-service-account=%s", impersonateServiceAccount))
	}

	return executeCommand("gcloud", args...)
}
