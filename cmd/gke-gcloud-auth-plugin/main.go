package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	accessTokenEnvVar       = "GCLOUD_ACCESS_TOKEN"
	accessTokenExpiryEnvVar = "GCLOUD_ACCESS_TOKEN_EXPIRY"
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

type pluginContext struct {
	googleDefaultTokenSource         func(ctx context.Context, scope ...string) (oauth2.TokenSource, error)
	gcloudConfigOutput               func() ([]byte, error)
	k8sStartingConfig                func(po *clientcmd.PathOptions) (*clientcmdapi.Config, error)
	clientcmdModifyConfig            func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error
	cachedTokenEnvVarInput           func() (string, string)
	useApplicationDefaultCredentials bool
}

func newPluginContext() *pluginContext {
	return &pluginContext{
		googleDefaultTokenSource:         google.DefaultTokenSource,
		k8sStartingConfig:                k8sStartingConfig,
		gcloudConfigOutput:               gcloudConfigOutput,
		clientcmdModifyConfig:            clientcmd.ModifyConfig,
		cachedTokenEnvVarInput:           cachedTokenEnvVarInput,
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
		klog.V(4).Infof("Failed to cache token ", err)
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
		fmt.Printf("error parsing gcloud output : %+v", err.Error())
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
	token, expiry := pc.cachedTokenEnvVarInput()

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
	po := clientcmd.NewDefaultPathOptions()
	startingConfig, err := pc.k8sStartingConfig(po)
	if err != nil {
		klog.V(4).Infof("Error getting starting config %v", err)
	}
	ctx := startingConfig.Contexts[startingConfig.CurrentContext]
	currAuthInfo, ok := startingConfig.AuthInfos[ctx.AuthInfo]
	if !ok {
		klog.V(4).Infof("curr auth info not found")
	}

	if currAuthInfo.Exec != nil {
		if currAuthInfo.Exec.Env == nil {
			currAuthInfo.Exec.Env = []clientcmdapi.ExecEnvVar{}
		}
		appendExecEnv(&currAuthInfo.Exec.Env, accessTokenEnvVar, accessToken)
		appendExecEnv(&currAuthInfo.Exec.Env, accessTokenExpiryEnvVar, expiry.Format(time.RFC3339Nano))
	}

	return pc.clientcmdModifyConfig(po, *startingConfig, false)
}

func k8sStartingConfig(po *clientcmd.PathOptions) (*clientcmdapi.Config, error) {
	return po.GetStartingConfig()
}

func cachedTokenEnvVarInput() (string, string) {
	return os.Getenv(accessTokenEnvVar), os.Getenv(accessTokenExpiryEnvVar)
}

func appendExecEnv(envs *[]clientcmdapi.ExecEnvVar, name, value string) {
	found := false
	for i := range *envs {
		if (*envs)[i].Name == name {
			(*envs)[i].Value = value
			found = true
		}
	}
	if !found {
		*envs = append(*envs, clientcmdapi.ExecEnvVar{name, value})
	}
}

func formatToJSON(i interface{}) (string, error) {
	s, err := json.MarshalIndent(i, "", "    ")
	if err != nil {
		return "", err
	}
	return string(s), nil
}
