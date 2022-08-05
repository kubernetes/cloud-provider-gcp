package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// gcloudEdgeCloudTokenProvider provides gcloud edge-cloud tokens.
type gcloudEdgeCloudTokenProvider struct {
	location    string
	clusterName string
	getTokenRaw func(location string, clusterName string) ([]byte, error)
}

// gcloudEdgeCloudToken holds types unmarshaled from the edge cloud access token in json format
type gcloudEdgeCloudToken struct {
	AccessToken string    `json:"accessToken"`
	TokenExpiry time.Time `json:"expireTime"`
}

func (p *gcloudEdgeCloudTokenProvider) token() (string, *time.Time, error) {
	edgeCloudTokenBytes, err := p.getTokenRaw(p.clusterName, p.location)
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

func getGcloudEdgeCloudTokenRaw(clusterName string, location string) ([]byte, error) {
	return executeCommand("gcloud", "edge-cloud", "container", "clusters", "print-access-token", clusterName, fmt.Sprintf("--location=%s", location), "--format=json")
}
