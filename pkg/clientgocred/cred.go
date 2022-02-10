// package clientgocred prints an ExecCredential object to stdout. The ExecCredential
// object is filled with an access_token either from gcloud or from application
// default credentials. This is defined by Client-go Credential plugins:
// https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins
// This library can be used with GKE Clusters for use with kubectl and custom
// k8s clients.
package clientgocred

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/natefinch/atomic"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthv1b1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

const (
	// cacheFileName is the file which stores the access tokens. This file is
	// co-located with kubeconfig file. This file is deleted by get-credentials
	// code in "gcloud container clusters" upon every invocation and is recreated
	// by gke-gcloud-auth-plugin.
	cacheFileName = "gke_gcloud_auth_plugin_cache"
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

// cache is the struct that gets cached in the cache file in json format.
// {
//    "current_context": "gke_user-gke-dev_us-central1_autopilot-cluster-11",
//    "access_token": "ya29.A0ARrdaM8WL....G0xYXGIQNPi5WvHe07ia4Gs",
//    "token_expiry": "2022-01-27T08:27:52Z"
// }
// The current_context helps us cache tokens by context(cluster) similar to how
// this was done for Authprovider in kubeconfig.
type cache struct {
	// CurrentContext refers to which context the token was last retrieved for. If
	// currentContext in kubeconfig is changed, the current cached access token is invalidated.
	CurrentContext string `json:"current_context"`
	// AccessToken is gcloud access token
	AccessToken string `json:"access_token"`
	// TokenExpiry is gcloud access token's expiry.
	TokenExpiry string `json:"token_expiry"`
}

// plugin holds data to be passed around (eg: useApplicationDefaultCredentials)
// as well as methods that may needs to be mocked in test scenarios.
type plugin struct {
	googleDefaultTokenSource         func(ctx context.Context, scope ...string) (oauth2.TokenSource, error)
	readGcloudConfigRaw              func() ([]byte, error)
	k8sStartingConfig                func() (*clientcmdapi.Config, error)
	getCachedToken                   func(pc *plugin) (string, string, error)
	writeCacheFile                   func(content string) error
	useApplicationDefaultCredentials bool
}

func newPlugin(options *Options) *plugin {
	return &plugin{
		googleDefaultTokenSource:         google.DefaultTokenSource,
		k8sStartingConfig:                k8sStartingConfig,
		readGcloudConfigRaw:              readGcloudConfigRaw,
		getCachedToken:                   getCachedToken,
		writeCacheFile:                   writeCacheFile,
		useApplicationDefaultCredentials: options.UseApplicationDefaultCredentials,
	}
}

// Options struct inputs to PrintCred
type Options struct {
	UseApplicationDefaultCredentials bool
}

// PrintCred prints ExecCredential to stdout to be consumed by kubectl to connect to GKE Clusters
// {
//    "kind": "ExecCredential",
//    "apiVersion": "client.authentication.k8s.io/v1beta1",
//    "spec": {
//        "interactive": false
//    },
//    "status": {
//        "expirationTimestamp": "2022-01-27T07:10:46Z",
//        "token": "ya29.A0ARrda.......0jDi8weH-36jJNru6Ps"
//    }
// }
func PrintCred(options *Options) error {
	if options == nil {
		options = &Options{
			UseApplicationDefaultCredentials: false,
		}
	}
	pc := newPlugin(options)

	ec, err := pc.execCredential()
	if err != nil {
		return err
	}

	ecStr, err := formatToJSON(ec)
	if err != nil {
		return err
	}

	if _, err := fmt.Print(ecStr); err != nil {
		return fmt.Errorf("unable to write ExecCredential to stdout: %w", err)
	}

	return nil
}

// execCredential return an object of type ExecCredential which
// holds a bearer token to authenticate to GKE.
func (pc *plugin) execCredential() (*clientauthv1b1.ExecCredential, error) {
	token, expiry, err := pc.accessToken()
	if err != nil {
		return nil, err
	}

	return &clientauthv1b1.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ExecCredential",
			APIVersion: "client.authentication.k8s.io/v1beta1",
		},
		Status: &clientauthv1b1.ExecCredentialStatus{
			Token:               token,
			ExpirationTimestamp: expiry,
		},
	}, nil
}

// accessToken return either the ApplicationDefaultCredentials or the gcloudAccessToken
func (pc *plugin) accessToken() (string, *metav1.Time, error) {
	if !pc.useApplicationDefaultCredentials {
		if token, expiry, err := pc.gcloudAccessToken(); err == nil {
			return token, expiry, nil
		}
		// if err is not nil, fall back to returning Application default credentials
	}
	return pc.defaultAccessToken()
}

// gcloudAccessToken returns a cached token if the token is not expired. If the token is
// expired, it gets a new access token by invoking gcloud command, caches the new token
// and returns the token.
func (pc *plugin) gcloudAccessToken() (string, *metav1.Time, error) {
	if token, expiry, err := pc.getCachedGcloudAccessToken(); err == nil {
		return token, expiry, nil
	} else {
		// log and ignore error; move on to getting a new token from gcloud
		klog.V(4).Infof("Getting cached gcloud access token failed with error: %v", err)
	}

	gc, err := pc.readGcloudConfig()
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

	if err := pc.writeGcloudAccessTokenToCache(token, gc.Credential.TokenExpiry); err != nil {
		// log and ignore error as writing to cache is best effort
		klog.V(4).Infof("Failed to write gcloud access token to cache with error: %v", err)
	}

	return token, &metav1.Time{Time: gc.Credential.TokenExpiry}, nil
}

func (pc *plugin) defaultAccessToken() (string, *metav1.Time, error) {
	var tok *oauth2.Token

	// Retries (max 4 retries with approx delay 10*ms+jitter setup) help get around occasional network glitches
	err := retry.OnError(retry.DefaultBackoff, func(err error) bool { return true }, func() error {
		ts, err := pc.googleDefaultTokenSource(context.Background(), defaultScopes...)
		if err != nil {
			return fmt.Errorf("cannot construct google default token source: %w", err)
		}

		tok, err = ts.Token()
		if err != nil {
			return fmt.Errorf("cannot retrieve default token from google default token source: %w", err)
		}

		return nil
	})
	if err != nil {
		return "", nil, err
	}

	return tok.AccessToken, &metav1.Time{Time: tok.Expiry}, nil
}

// readGcloudConfig returns an object which represents gcloud config output
func (pc *plugin) readGcloudConfig() (*gcloudConfiguration, error) {
	gcloudConfigbytes, err := pc.readGcloudConfigRaw()
	if err != nil {
		return nil, err
	}
	var gc gcloudConfiguration
	if err := json.Unmarshal(gcloudConfigbytes, &gc); err != nil {
		return nil, fmt.Errorf("error parsing gcloud output: %w", err)
	}

	return &gc, nil
}

func readGcloudConfigRaw() ([]byte, error) {
	cmd := exec.Command("gcloud", "config", "config-helper", "--format=json")
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("while executing gcloud config config-helper: %w", err)
	}
	return stdoutBuffer.Bytes(), nil
}

func (pc *plugin) getCachedGcloudAccessToken() (string, *metav1.Time, error) {
	token, expiry, err := pc.getCachedToken(pc)
	if err != nil {
		return "", nil, err
	}

	expiryTimeStamp, err := time.Parse(time.RFC3339Nano, expiry)
	if err != nil {
		return "", nil, err
	}

	if token == "" {
		return "", nil, fmt.Errorf("cached token is empty")
	}
	// Check if the cached token is valid for 10 secs (this check comes from oauth2 token.Valid())
	if time.Now().After(expiryTimeStamp.Add(-10 * time.Second)) {
		return "", nil, fmt.Errorf("cached token is expiring in 10 seconds")
	}

	return token, &metav1.Time{Time: expiryTimeStamp}, nil
}

func (pc *plugin) writeGcloudAccessTokenToCache(accessToken string, expiry time.Time) error {
	startingConfig, err := pc.k8sStartingConfig()
	if err != nil {
		return fmt.Errorf("error getting starting config: %w", err)
	}

	c := cache{
		CurrentContext: startingConfig.CurrentContext,
		AccessToken:    accessToken,
		TokenExpiry:    expiry.Format(time.RFC3339Nano),
	}

	formatted, err := formatToJSON(c)
	if err != nil {
		return err
	}

	return pc.writeCacheFile(formatted)
}

func writeCacheFile(content string) error {
	cacheFilePath := getCacheFilePath()
	// File is atomically written with 0600 - the same permissions as ~/.kube/config file.
	// ls ~/.kube/ -al
	// -rw-------  1 username primarygroup 2836 Jan 27 08:00 config
	// -rw-------  1 username primarygroup  327 Jan 27 08:00 gke_gcloud_auth_plugin_cache
	return atomic.WriteFile(cacheFilePath, strings.NewReader(content))
}

func getCachedToken(pc *plugin) (string, string, error) {
	cacheFilePath := getCacheFilePath()
	content, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		return "", "", err
	}
	var c cache
	if err = json.Unmarshal(content, &c); err != nil {
		return "", "", err
	}

	startingConfig, err := pc.k8sStartingConfig()
	if err != nil {
		return "", "", err
	}
	// If current context is not the same as what the cached access token was
	// generated for, then consider the current access token invalid.
	if c.CurrentContext != startingConfig.CurrentContext {
		return "", "", err
	}

	return c.AccessToken, c.TokenExpiry, err
}

func k8sStartingConfig() (*clientcmdapi.Config, error) {
	po := clientcmd.NewDefaultPathOptions()
	return po.GetStartingConfig()
}

func getCacheFilePath() string {
	po := clientcmd.NewDefaultPathOptions()
	kubeconfig := po.GetDefaultFilename()
	dir := filepath.Dir(kubeconfig)
	cacheFilePath := filepath.Join(dir, cacheFileName)
	return cacheFilePath
}

func formatToJSON(i interface{}) (string, error) {
	s, err := json.MarshalIndent(i, "", "    ")
	if err != nil {
		return "", err
	}
	return string(s), nil
}
