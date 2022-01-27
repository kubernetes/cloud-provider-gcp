package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauth "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
)

const (
	cacheFileName = "plugin_cache"
)

var (
	// defaultScopes:
	// - cloud-platform is the base scope to authenticate to GCP.
	// - userinfo.email is used to authenticate to GKE APIs with gserviceaccount
	//   email instead of numeric uniqueID.
	defaultScopes = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email"}

	useAdcPtr = pflag.Bool("use_application_default_credentials", false, "returns exec credential filled with application default credentials.")
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

type cache struct {
	CurrentContext string `json:"current_context"`
	AccessToken    string `json:"access_token"`
	TokenExpiry    string `json:"token_expiry"`
}

type pluginContext struct {
	googleDefaultTokenSource         func(ctx context.Context, scope ...string) (oauth2.TokenSource, error)
	gcloudConfigOutput               func() ([]byte, error)
	k8sStartingConfig                func() (*clientcmdapi.Config, error)
	cachedToken                      func(pc *pluginContext) (string, string)
	writeCacheFile                   func(content string) error
	useApplicationDefaultCredentials bool
}

func newPluginContext() *pluginContext {
	return &pluginContext{
		googleDefaultTokenSource:         google.DefaultTokenSource,
		k8sStartingConfig:                k8sStartingConfig,
		gcloudConfigOutput:               gcloudConfigOutput,
		cachedToken:                      cachedToken,
		writeCacheFile:                   writeCacheFile,
		useApplicationDefaultCredentials: *useAdcPtr,
	}
}

func main() {
	pflag.Parse()
	verflag.PrintAndExitIfRequested()
	pc := newPluginContext()

	ec, err := execCredential(pc)
	if err != nil {
		klog.Fatalf("unable to retrieve access token for GKE. Error : %v", err)
	}

	ecStr, err := formatToJSON(ec)
	if err != nil {
		klog.Fatalf("unable to convert ExecCredential object to json format. Error :%v", err)
	}
	// Print output to be consumed by kubectl
	fmt.Print(ecStr)
}

// ExecCredential return an object of type ExecCredential which
// holds a bearer token to authenticate to GKE.
func execCredential(pc *pluginContext) (*clientauth.ExecCredential, error) {
	token, expiry, err := accessToken(pc)
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

func accessToken(pc *pluginContext) (string, *meta.Time, error) {
	if !pc.useApplicationDefaultCredentials {
		token, expiry, err := gcloudAccessToken(pc)
		if err == nil {
			return token, expiry, nil
		}
		// if err is not nil, fall back to returning Application default credentials
	}
	return defaultAccessToken(pc)
}

func gcloudAccessToken(pc *pluginContext) (string, *meta.Time, error) {
	if token, expiry, ok := cachedGcloudAccessToken(pc); ok {
		return token, expiry, nil
	}

	gc, err := newGcloudConfig(pc)
	if err != nil {
		return "", nil, err
	}

	if err := cacheGcloudAccessToken(pc, gc.Credential.AccessToken, gc.Credential.TokenExpiry); err != nil {
		//fmt.Printf("caching failed: %+v", err)
		klog.V(4).Infof("Failed to cache token %+v", err)
	}

	return gc.Credential.AccessToken, &meta.Time{Time: gc.Credential.TokenExpiry}, nil
}

func defaultAccessToken(pc *pluginContext) (string, *meta.Time, error) {
	ts, err := pc.googleDefaultTokenSource(context.Background(), defaultScopes...)
	if err != nil {
		return "", nil, fmt.Errorf("cannot construct google default token source: %v", err)
	}

	tok, err := ts.Token()
	if err != nil {
		return "", nil, fmt.Errorf("cannot retrieve default token from google default token source: %v", err)
	}

	return tok.AccessToken, &meta.Time{Time: tok.Expiry}, nil
}

// newGcloudConfig returns an object which represents gcloud config output
func newGcloudConfig(pc *pluginContext) (*gcloudConfiguration, error) {
	gcloudConfigbytes, err := pc.gcloudConfigOutput()
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

func cachedGcloudAccessToken(pc *pluginContext) (string, *meta.Time, bool) {
	token, expiry := pc.cachedToken(pc)

	timeStamp, err := time.Parse(time.RFC3339Nano, expiry)
	if err != nil {
		klog.V(4).Infof("\nerror parsing time %+v\n\n", err)
		return "", nil, false
	}

	tok := &oauth2.Token{
		AccessToken: token,
		TokenType:   "Bearer",
		Expiry:      timeStamp,
	}

	if !tok.Valid() || tok.Expiry.IsZero() {
		klog.V(4).Infof("\nerror validating token %+v\n\n", tok)
		return "", nil, false
	}

	return tok.AccessToken, &meta.Time{tok.Expiry}, true
}

func cacheGcloudAccessToken(pc *pluginContext, accessToken string, expiry time.Time) error {
	startingConfig, err := pc.k8sStartingConfig()
	if err != nil {
		klog.V(4).Infof("Error getting starting config %v", err)
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
	return ioutil.WriteFile(cacheFilePath, []byte(content), 0600)
}

func cachedToken(pc *pluginContext) (string, string) {
	cacheFilePath := getCacheFilePath()
	content, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		//fmt.Printf("error reading file : %+v", err)
		return "", ""
	}
	var c cache
	if err := json.Unmarshal(content, &c); err != nil {
		return "", ""
	}

	startingConfig, err := pc.k8sStartingConfig()
	if err != nil {
		klog.V(4).Infof("Error getting starting config %v", err)
	}
	if c.CurrentContext != startingConfig.CurrentContext {
		return "", ""
	}

	return c.AccessToken, c.TokenExpiry
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
