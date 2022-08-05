package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// gcloudConfiguration holds types unmarshaled from gcloud config in json format
type gcloudConfiguration struct {
	Credential struct {
		AccessToken string    `json:"access_token"`
		TokenExpiry time.Time `json:"token_expiry"`
	} `json:"credential"`
	Configuration struct {
		Properties struct {
			Auth struct {
				AuthorizationTokenFile string `json:"authorization_token_file"`
			} `json:"auth"`
		} `json:"properties"`
	} `json:"configuration"`
}

// gcloudTokenProvider provides gcloud OAth 2.0 tokens.
type gcloudTokenProvider struct {
	readGcloudConfigRaw func() ([]byte, error)
	readFile            func(filename string) ([]byte, error)
}

// readGcloudConfig returns an object which represents gcloud config output
func (p *gcloudTokenProvider) readGcloudConfig() (*gcloudConfiguration, error) {
	gcloudConfigBytes, err := p.readGcloudConfigRaw()
	if err != nil {
		return nil, err
	}
	var gc gcloudConfiguration
	if err := json.Unmarshal(gcloudConfigBytes, &gc); err != nil {
		return nil, fmt.Errorf("error parsing gcloud output: %w", err)
	}

	return &gc, nil
}

func (p *gcloudTokenProvider) token() (string, *time.Time, error) {
	gc, err := p.readGcloudConfig()
	if err != nil {
		return "", nil, err
	}

	if gc.Credential.AccessToken == "" {
		return "", nil, fmt.Errorf("gcloud config config-helper returned an empty access token")
	}
	if gc.Credential.TokenExpiry.IsZero() {
		return "", nil, fmt.Errorf("failed to retrieve expiry time from gcloud config json object")
	}

	// Authorization Token File is not commonly used. Currently, this is for specific internal debugging scenarios.
	token := gc.Credential.AccessToken
	var authzTokenFile string
	var authzTokenBytes []byte
	if authzTokenFile = gc.Configuration.Properties.Auth.AuthorizationTokenFile; authzTokenFile != "" {
		authzTokenBytes, err = p.readFile(authzTokenFile)
		if err != nil {
			return "", nil, fmt.Errorf("gcloud config sets property auth/authorization_token_file, but can't read file at %s: %w", authzTokenFile, err)
		}
		token = fmt.Sprintf("iam-%s^%s", token, authzTokenBytes)
	}

	return token, &gc.Credential.TokenExpiry, nil
}

func (p *gcloudTokenProvider) useCache() bool { return true }

func readGcloudConfigRaw() ([]byte, error) {
	return executeCommand("gcloud", "config", "config-helper", "--format=json")
}
