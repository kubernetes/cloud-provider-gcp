package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/oauth2/google"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		"https://www.googleapis.com/auth/userinfo.email",
	}
)

// types unmarshaled from gcloud config in json format
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

var (
	useApplicationDefaultCredentials = pflag.Bool("use_application_default_credentials", false, "returns exec credential filled with application default credentials.")
)

func main() {
	pflag.Parse()
	verflag.PrintAndExitIfRequested()

	p := plugin{
		useApplicationDefaultCredentials: *useApplicationDefaultCredentials,
		readGcloudConfigRaw:              readGcloudConfigRaw,
		w:                                os.Stdout,
	}

	if err := p.run(); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to retrieve access token for GKE: %s", err)
		os.Exit(1)
	}
}

type plugin struct {
	useApplicationDefaultCredentials bool
	readGcloudConfigRaw              func() ([]byte, error)
	w                                io.Writer
}

func (p *plugin) run() error {
	creds, err := p.execCredential()
	if err != nil {
		return fmt.Errorf("unable to retrieve access token for GKE: %w", err)
	}

	out, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("unable to convert ExecCredential object to json format: %w", err)
	}

	if _, err := p.w.Write(out); err != nil {
		return fmt.Errorf("unable to write ExecCredential to stdout: %w", err)
	}

	return nil
}

// execCredential return an object of type ExecCredential which
// holds a bearer token to authenticate to GKE.
func (p *plugin) execCredential() (*clientauth.ExecCredential, error) {
	token, expiry, err := p.accessToken()
	if err != nil {
		return nil, err
	}

	return &clientauth.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ExecCredential",
			APIVersion: "client.authentication.k8s.io/v1beta1",
		},
		Status: &clientauth.ExecCredentialStatus{
			Token:               token,
			ExpirationTimestamp: expiry,
		},
	}, nil
}

func (p *plugin) accessToken() (string, *metav1.Time, error) {
	if !p.useApplicationDefaultCredentials {
		return p.gcloudAccessToken()
	}
	return p.defaultAccessToken()
}

func (p *plugin) gcloudAccessToken() (string, *metav1.Time, error) {
	gc, err := p.readGcloudConfig()
	if err != nil {
		return "", nil, err
	}
	if gc.Credential.AccessToken == "" {
		return "", nil, fmt.Errorf("failed to retrieve access token from gcloud config json object")
	}
	if gc.Credential.TokenExpiry.IsZero() {
		return "", nil, fmt.Errorf("failed to retrieve expiry time from gcloud config json object")
	}

	token := gc.Credential.AccessToken
	if authzTokenFile := gc.Configuration.Properties.Auth.AuthorizationTokenFile; authzTokenFile != "" {
		authzTokenBytes, err := ioutil.ReadFile(authzTokenFile)
		if err != nil {
			return "", nil, fmt.Errorf("gcloud config sets property auth/authorization_token_file, but can't read file at %s: %w", authzTokenFile, err)
		}
		token = fmt.Sprintf("iam-%s^%s", token, authzTokenBytes)
	}

	return token, &metav1.Time{Time: gc.Credential.TokenExpiry}, nil
}

func (p *plugin) defaultAccessToken() (string, *metav1.Time, error) {
	ts, err := google.DefaultTokenSource(context.Background(), defaultScopes...)
	if err != nil {
		return "", nil, fmt.Errorf("cannot construct google default token source: %w", err)
	}

	tok, err := ts.Token()
	if err != nil {
		return "", nil, fmt.Errorf("cannot retrieve default token from google default token source: %w", err)
	}

	return tok.AccessToken, &metav1.Time{Time: tok.Expiry}, nil
}

// readGcloudConfig returns an object which represents gcloud config output
func (p *plugin) readGcloudConfig() (*gcloudConfiguration, error) {
	gcRaw, err := p.readGcloudConfigRaw()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve gcloud config: %w", err)
	}

	var gc gcloudConfiguration
	if err := json.Unmarshal(gcRaw, &gc); err != nil {
		return nil, fmt.Errorf("error parsing gcloud output : %w", err)
	}

	return &gc, nil
}

func readGcloudConfigRaw() ([]byte, error) {
	cmd := exec.Command("gcloud", "config", "config-helper", "--format=json")

	var buf bytes.Buffer
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
