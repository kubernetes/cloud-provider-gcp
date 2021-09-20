package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/oauth2/google"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauth "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"k8s.io/component-base/version/verflag"
)

var (
	// defaultScopes:
	// - cloud-platform is the base scope to authenticate to GCP.
	// - userinfo.email is used to authenticate to GKE APIs with gserviceaccount
	//   email instead of numeric uniqueID.
	defaultScopes = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email"}
)

type credential struct {
	AccessToken string                 `json:"access_token"`
	TokenExpiry time.Time              `json:"token_expiry"`
	X           map[string]interface{} `json:"-"` // Rest of the fields should go here.
}

// gcloudConfiguration is the struct unmarshalled
// from gcloud config in json format
type gcloudConfiguration struct {
	Credential credential             `json:"credential"`
	X          map[string]interface{} `json:"-"` // Rest of the fields should go here.
}

var (
	useAdcPtr = pflag.Bool("use_application_default_credentials", false, "returns exec credential filled with application default credentials.")
)

func main() {
	pflag.Parse()
	verflag.PrintAndExitIfRequested()

	ec, err := execCredential()
	if err != nil {
		msg := fmt.Errorf("unable to retrieve access token for GKE. Error : %v", err)
		panic(msg)
	}

	ecStr, err := formatToJSON(ec)
	if err != nil {
		msg := fmt.Errorf("unable to convert ExecCredential object to json format. Error :%v", err)
		panic(msg)
	}
	fmt.Print(ecStr)
}

// ExecCredential return an object of type ExecCredential which
// holds a bearer token to authenticate to GKE.
func execCredential() (*clientauth.ExecCredential, error) {
	token, expiry, err := accessToken()
	if err != nil {
		return nil, err
	}

	return &clientauth.ExecCredential{
		TypeMeta: meta.TypeMeta{
			Kind:       "ExecCredential",
			APIVersion: "client.authentication.k8s.io/v1beta1",
		},
		Status: &clientauth.ExecCredentialStatus{
			Token:               token,
			ExpirationTimestamp: expiry,
		},
	}, nil
}

func accessToken() (string, *meta.Time, error) {
	if !*useAdcPtr {
		token, expiry, err := gcloudAccessToken()
		if err == nil {
			return token, expiry, nil
		}
	}
	return defaultAccessToken()
}

func gcloudAccessToken() (string, *meta.Time, error) {
	gc, err := retrieveGcloudConfig()
	if err != nil {
		return "", nil, err
	}

	return gc.Credential.AccessToken, &meta.Time{Time: gc.Credential.TokenExpiry}, nil
}

func defaultAccessToken() (string, *meta.Time, error) {
	ts, err := google.DefaultTokenSource(context.Background(), defaultScopes...)
	if err != nil {
		return "", nil, fmt.Errorf("cannot construct google default token source: %v", err)
	}

	tok, err := ts.Token()
	if err != nil {
		return "", nil, fmt.Errorf("cannot retrieve default token from google default token source: %v", err)
	}

	return tok.AccessToken, &meta.Time{Time: tok.Expiry}, nil
}

// retrieveGcloudConfig returns an object which represents gcloud config output
func retrieveGcloudConfig() (*gcloudConfiguration, error) {
	gcloudConfigbytes, err := gcloudConfigOutput()
	if err != nil {
		return nil, err
	}
	var gc gcloudConfiguration
	if err := json.Unmarshal(gcloudConfigbytes, &gc); err != nil {
		return nil, fmt.Errorf("error parsing gcloud output : %+v", err.Error())
	}

	return &gc, nil
}

func gcloudConfigOutput() ([]byte, error) {
	cmd := exec.Command("gcloud", "config", "config-helper", "--format=json")
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve gcloud config. Error message: %s, stdout: %s, stderr: %s", err.Error(), stdoutBuffer.String(), stderrBuffer.String())
	}
	return stdoutBuffer.Bytes(), nil
}

func formatToJSON(i interface{}) (string, error) {
	s, err := json.MarshalIndent(i, "", "    ")
	if err != nil {
		return "", err
	}
	return string(s), nil
}
