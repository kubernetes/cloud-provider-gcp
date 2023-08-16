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
	getTokenRaw               func(args []string) ([]byte, error)
}

// gcloudEdgeCloudToken holds types unmarshaled from the edge cloud access token in json format
type gcloudEdgeCloudToken struct {
	AccessToken string    `json:"accessToken"`
	TokenExpiry time.Time `json:"expireTime"`
}

func (p *gcloudEdgeCloudTokenProvider) token() (string, *time.Time, error) {
	gcloudArgs := p.getGcloudArgs()
	edgeCloudTokenBytes, err := p.getTokenRaw(gcloudArgs)
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

func (p *gcloudEdgeCloudTokenProvider) getExtraArgs() []string {
	args := []string{p.clusterName, fmt.Sprintf("--project=%s", p.project), fmt.Sprintf("--location=%s", p.location), "--format=json"}
	if p.impersonateServiceAccount != "" {
		args = append(args, fmt.Sprintf("--impersonate-service-account=%s", p.impersonateServiceAccount))
	}
	return args
}

func (p *gcloudEdgeCloudTokenProvider) getGcloudArgs() []string {
	args := []string{"edge-cloud", "container", "clusters", "print-access-token"}
	args = append(args, p.getExtraArgs()...)
	return args
}

func getGcloudEdgeCloudTokenRaw(args []string) ([]byte, error) {
	return executeCommand("gcloud", args...)
}
