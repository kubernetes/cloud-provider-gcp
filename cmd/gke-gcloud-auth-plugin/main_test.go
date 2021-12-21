package main

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type mockTokenSource struct{}

func (*mockTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		TokenType:   "bearer",
		AccessToken: "default_access_token",
		Expiry:      time.Now().Add(time.Hour),
	}, nil
}

func TestExecCredential(t *testing.T) {
	testCases := []struct {
		testName      string
		pc            *pluginContext
		expectedToken string
	}{
		{
			testName: "ApplicationDefaultCredentialsSetToTrue",
			pc: &pluginContext{
				googleDefaultTokenSource:         fakeDefaultTokenSource,
				gcloudConfigOutput:               fakeGcloudConfigOutput,
				k8sStartingConfig:                fakeK8sStartingConfig,
				clientcmdModifyConfig:            fakeModifyConfig,
				cachedTokenEnvVarInput:           func() (string, string) { return "", "" },
				useApplicationDefaultCredentials: true,
			},
			expectedToken: "default_access_token",
		},
		{
			testName: "NewGcloudAccessToken",
			pc: &pluginContext{
				googleDefaultTokenSource:         nil,
				gcloudConfigOutput:               fakeGcloudConfigOutput,
				clientcmdModifyConfig:            fakeModifyConfig,
				k8sStartingConfig:                fakeK8sStartingConfig,
				cachedTokenEnvVarInput:           func() (string, string) { return "", "" },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "ya29.gcloud_token",
		},
		{
			testName: "GcloudAccessTokenFailureFallbackToADC",
			pc: &pluginContext{
				googleDefaultTokenSource: fakeDefaultTokenSource,
				gcloudConfigOutput: func() ([]byte, error) {
					return []byte("bad token string"), nil
				},
				k8sStartingConfig:                fakeK8sStartingConfig,
				clientcmdModifyConfig:            fakeModifyConfig,
				cachedTokenEnvVarInput:           func() (string, string) { return "", "" },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "default_access_token",
		},
		{
			testName: "GcloudCommandFailureFailureFallbackToADC",
			pc: &pluginContext{
				googleDefaultTokenSource: fakeDefaultTokenSource,
				gcloudConfigOutput: func() ([]byte, error) {
					return []byte("gcloud_command_failure"), errors.New("gcloud command failure")
				},
				k8sStartingConfig:                fakeK8sStartingConfig,
				clientcmdModifyConfig:            fakeModifyConfig,
				cachedTokenEnvVarInput:           func() (string, string) { return "", "" },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "default_access_token",
		},
		{
			testName: "CachedTokenIsValid",
			pc: &pluginContext{
				googleDefaultTokenSource:         nil,
				gcloudConfigOutput:               nil,
				k8sStartingConfig:                fakeK8sStartingConfig,
				clientcmdModifyConfig:            fakeModifyConfig,
				cachedTokenEnvVarInput:           func() (string, string) { return "cached_token", time.Now().Add(time.Hour).Format(time.RFC3339Nano) },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "cached_token",
		},
		{
			testName: "CachedTokenInvalid",
			pc: &pluginContext{
				googleDefaultTokenSource: nil,
				gcloudConfigOutput:       fakeGcloudConfigOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				clientcmdModifyConfig:    fakeModifyConfig,
				cachedTokenEnvVarInput: func() (string, string) {
					return "cached_token_invalid", time.Now().Add(-time.Hour).Format(time.RFC3339Nano)
				},
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "ya29.gcloud_token",
		},
		{
			testName: "CachedTokenOverwrite",
			pc: &pluginContext{
				googleDefaultTokenSource: nil,
				gcloudConfigOutput:       fakeGcloudConfigOutput,
				k8sStartingConfig:        fakeK8sStartingConfigWithEnvVars,
				clientcmdModifyConfig:    fakeModifyConfig,
				cachedTokenEnvVarInput: func() (string, string) {
					return "cached_token_expired", time.Now().Add(-time.Hour).Format(time.RFC3339Nano)
				},
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "ya29.gcloud_token",
		},
		{
			testName: "CachingFails",
			pc: &pluginContext{
				googleDefaultTokenSource: nil,
				gcloudConfigOutput:       fakeGcloudConfigOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				clientcmdModifyConfig: func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error {
					return errors.New("caching error")
				},
				cachedTokenEnvVarInput:           func() (string, string) { return "cached_token", time.Now().Add(-time.Hour).Format(time.RFC3339Nano) },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "ya29.gcloud_token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			ec, err := execCredential(tc.pc)
			if err != nil {
				t.Fatalf("err should be nil")
			}
			if ec.Status == nil {
				t.Fatalf("ec.Status should not be nil")
			}
			if ec.Status.Token != tc.expectedToken {
				t.Errorf("want %s, got %s", tc.expectedToken, ec.Status.Token)
			}
			_, err = formatToJSON(ec)
			if err != nil {
				t.Fatalf("err should be nil")
			}

		})
	}
}

func fakeDefaultTokenSource(ctx context.Context, scope ...string) (oauth2.TokenSource, error) {
	return &mockTokenSource{}, nil
}

func fakeGcloudConfigOutput() ([]byte, error) {
	fakeOutput := `{
  "configuration": {
    "active_configuration": "default",
    "properties": {
      "compute": {
        "zone": "us-central1-c"
      },
      "container": {
        "cluster": "kirantumkur-cluster",
        "use_application_default_credentials": "false"
      },
      "core": {
        "account": "kirantumkur@google.com",
        "disable_usage_reporting": "False",
        "project": "kirantumkur-gke-dev"
      }
    }
  },
  "credential": {
    "access_token": "ya29.gcloud_token",
    "token_expiry": "2021-12-21T01:07:51Z"
  },
  "sentinels": {
    "config_sentinel": "/usr/local/google/home/kirantumkur/.config/gcloud/config_sentinel"
  }
}`
	return []byte(fakeOutput), nil
}

func fakeK8sStartingConfigWithEnvVars(po *clientcmd.PathOptions) (*clientcmdapi.Config, error) {
	c, err := fakeK8sStartingConfig(po)
	if err != nil {
		return nil, err
	}
	c.AuthInfos["gke_user-gke-dev_us-east1-b_cluster-1"].Exec.Env = []clientcmdapi.ExecEnvVar{}
	appendExecEnv(&c.AuthInfos["gke_user-gke-dev_us-east1-b_cluster-1"].Exec.Env, accessTokenEnvVar, "cached_token_expired")
	appendExecEnv(&c.AuthInfos["gke_user-gke-dev_us-east1-b_cluster-1"].Exec.Env, accessTokenExpiryEnvVar, time.Now().Add(-time.Hour).Format(time.RFC3339Nano))
	return c, nil
}

func fakeK8sStartingConfig(po *clientcmd.PathOptions) (*clientcmdapi.Config, error) {
	return &clientcmdapi.Config{
		Kind:        "Config",
		APIVersion:  "v1",
		Preferences: clientcmdapi.Preferences{},
		Clusters:    nil,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"gke_user-gke-dev_us-east1-b_cluster-1": &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Env: nil,
				},
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"gke_user-gke-dev_us-east1-b_cluster-1": &clientcmdapi.Context{
				Cluster:  "gke_user-gke-dev_us-east1-b_cluster-1",
				AuthInfo: "gke_user-gke-dev_us-east1-b_cluster-1",
			},
		},
		CurrentContext: "gke_user-gke-dev_us-east1-b_cluster-1",
		Extensions:     nil,
	}, nil
}

func fakeModifyConfig(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error {
	return nil
}
