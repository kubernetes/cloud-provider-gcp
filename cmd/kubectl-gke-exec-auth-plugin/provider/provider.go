package provider

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/oauth2/google"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauth "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
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

// ExecCredential return an object of type ExecCredential which
// holds a bearer token to authenticate to GKE.
func ExecCredential() (*clientauth.ExecCredential, error) {
	token, err := accessToken()
	if err != nil {
		return nil, err
	}

	ec := &clientauth.ExecCredential{
		TypeMeta: meta.TypeMeta{
			Kind:       "ExecCredential",
			APIVersion: "client.authentication.k8s.io/v1beta1",
		},
		Status: &clientauth.ExecCredentialStatus{
			Token: token,
		},
	}

	return ec, nil
}

func accessToken() (string, error) {
	token, err := gcloudAccessToken()
	if err == nil {
		return token, nil
	}
	return defaultAccessToken()
}

func gcloudAccessToken() (string, error) {
	cmd := exec.Command("gcloud", "config", "config-helper", "--format=get(credential.access_token)")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	token := string(output)
	token = strings.Trim(token, "\n")

	return token, nil
}

func defaultAccessToken() (string, error) {
	ts, err := google.DefaultTokenSource(context.Background(), defaultScopes...)
	if err != nil {
		return "", fmt.Errorf("cannot construct google default token source: %v", err)
	}

	tok, err := ts.Token()
	if err != nil {
		return "", fmt.Errorf("cannot retrieve default token from google default token source: %v", err)
	}
	at := tok.AccessToken
	return at, nil
}
