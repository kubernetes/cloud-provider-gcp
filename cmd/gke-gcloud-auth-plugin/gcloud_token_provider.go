package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"k8s.io/klog/v2"
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
	readGcloudConfigRaw       func(args []string) ([]byte, error)
	readFile                  func(filename string) ([]byte, error)
	account                   string
	project                   string
	impersonateServiceAccount string
}

// readGcloudConfig returns an object which represents gcloud config output
func (p *gcloudTokenProvider) readGcloudConfig(extraArgs []string) (*gcloudConfiguration, error) {
	gcloudConfigBytes, err := p.readGcloudConfigRaw(extraArgs)
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
	cloudsdkAuthAccessToken := os.Getenv(cloudsdkAuthAccessEnvVar)
	if cloudsdkAuthAccessToken != "" {
		klog.V(4).Infof("Returning token from Environment Variable CLOUDSDK_AUTH_ACCESS_TOKEN as it is populated")
		return cloudsdkAuthAccessToken, &time.Time{}, nil
	}
	gcloudArgs := p.getGcloudArgs()
	gc, err := p.readGcloudConfig(gcloudArgs)
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

func (p *gcloudTokenProvider) useCache() bool {
	cloudsdkAuthAccessToken := os.Getenv(cloudsdkAuthAccessEnvVar)
	if cloudsdkAuthAccessToken != "" {
		klog.V(4).Infof("cache is not being used as %s is populated", cloudsdkAuthAccessEnvVar)
		return false
	}
	return true
}

func (p *gcloudTokenProvider) getExtraArgs() []string {
	extraArgs := make([]string, 0, 3)
	if p.project != "" {
		extraArgs = append(extraArgs, fmt.Sprintf("--project=%s", p.project))
	}
	if p.account != "" {
		extraArgs = append(extraArgs, fmt.Sprintf("--account=%s", p.account))
	}
	if p.impersonateServiceAccount != "" {
		extraArgs = append(extraArgs, fmt.Sprintf("--impersonate-service-account=%s", p.impersonateServiceAccount))
	}
	return extraArgs
}

func (p *gcloudTokenProvider) getGcloudArgs() []string {
	args := []string{
		"config",
		"config-helper",
		"--format=json",
	}
	args = append(args, p.getExtraArgs()...)
	return args
}

func readGcloudConfigRaw(args []string) ([]byte, error) {
	klog.V(4).Infof("Executing gcloud command with args: %v", args)
	return executeCommand("gcloud", args...)
}
