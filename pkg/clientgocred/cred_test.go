package clientgocred

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauth "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
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
		pc            *plugin
		expectedToken string
	}{
		{
			testName: "ApplicationDefaultCredentialsSetToTrue",
			pc: &plugin{
				googleDefaultTokenSource:         fakeDefaultTokenSource,
				readGcloudConfigRaw:              fakeGcloudConfigOutput,
				k8sStartingConfig:                fakeK8sStartingConfig,
				getCachedToken:                   func(pc *plugin) (string, string, error) { return "", "", nil },
				writeCacheFile:                   func(content string) error { return nil },
				useApplicationDefaultCredentials: true,
			},
			expectedToken: "default_access_token",
		},
		{
			testName: "NewGcloudAccessToken",
			pc: &plugin{
				googleDefaultTokenSource:         nil,
				readGcloudConfigRaw:              fakeGcloudConfigOutput,
				k8sStartingConfig:                fakeK8sStartingConfig,
				getCachedToken:                   func(pc *plugin) (string, string, error) { return "", "", nil },
				writeCacheFile:                   func(content string) error { return nil },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "ya29.gcloud_token",
		},
		{
			testName: "GcloudAccessTokenFailureFallbackToADC",
			pc: &plugin{
				googleDefaultTokenSource: fakeDefaultTokenSource,
				readGcloudConfigRaw: func() ([]byte, error) {
					return []byte("bad token string"), nil
				},
				k8sStartingConfig:                fakeK8sStartingConfig,
				getCachedToken:                   func(pc *plugin) (string, string, error) { return "", "", nil },
				writeCacheFile:                   func(content string) error { return nil },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "default_access_token",
		},
		{
			testName: "GcloudCommandFailureFallbackToADC",
			pc: &plugin{
				googleDefaultTokenSource: fakeDefaultTokenSource,
				readGcloudConfigRaw: func() ([]byte, error) {
					return []byte("gcloud_command_failure"), errors.New("gcloud command failure")
				},
				k8sStartingConfig:                fakeK8sStartingConfig,
				getCachedToken:                   func(pc *plugin) (string, string, error) { return "", "", nil },
				writeCacheFile:                   func(content string) error { return nil },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "default_access_token",
		},
		{
			testName: "CachedTokenIsValid",
			pc: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      nil,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCachedToken: func(pc *plugin) (string, string, error) {
					return "cached_token", time.Now().Add(time.Hour).Format(time.RFC3339Nano), nil
				},
				writeCacheFile:                   func(content string) error { return nil },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "cached_token",
		},
		{
			testName: "CachedTokenInvalid",
			pc: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCachedToken: func(pc *plugin) (string, string, error) {
					return "cached_token_invalid", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), nil
				},
				writeCacheFile:                   func(content string) error { return nil },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "ya29.gcloud_token",
		},
		{
			testName: "CachedTokenOverwrite",
			pc: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCachedToken: func(pc *plugin) (string, string, error) {
					return "cached_token_expired", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), nil
				},
				writeCacheFile:                   func(content string) error { return fmt.Errorf("error writing to file") },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "ya29.gcloud_token",
		},
		{
			testName: "CachingFails",
			pc: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCachedToken: func(pc *plugin) (string, string, error) {
					return "cached_token", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), nil
				},
				writeCacheFile:                   func(content string) error { return nil },
				useApplicationDefaultCredentials: false,
			},
			expectedToken: "ya29.gcloud_token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			ec, err := tc.pc.execCredential()
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

func TestGcloudPluginWithAuthorizationToken(t *testing.T) {
	tokenFile, err := ioutil.TempFile("", "auth-token-test*")
	if err != nil {
		t.Fatalf("error creating temp auth-token file: %v", err)
	}
	defer os.Remove(tokenFile.Name())

	if _, err := io.WriteString(tokenFile, "authz-t0k3n"); err != nil {
		t.Fatal(err)
	}

	newYears := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

	gcloudConfig := fmt.Sprintf(`
{
  "configuration": {
    "active_configuration": "inspect-mikedanese-k8s",
    "properties": {
      "auth": {
        "authorization_token_file": "%s"
      }
    }
  },
  "credential": {
    "access_token": "ya29.t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z"
  }
}
`, tokenFile.Name())

	wantStatus := &clientauth.ExecCredentialStatus{
		Token:               "iam-ya29.t0k3n^authz-t0k3n",
		ExpirationTimestamp: &metav1.Time{Time: newYears},
	}

	p := &plugin{
		googleDefaultTokenSource: nil,
		readGcloudConfigRaw: func() ([]byte, error) {
			return []byte(gcloudConfig), nil
		},
		k8sStartingConfig: fakeK8sStartingConfig,
		getCachedToken: func(pc *plugin) (string, string, error) {
			return "cached_token_invalid", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), nil
		},
		writeCacheFile:                   func(content string) error { return nil },
		useApplicationDefaultCredentials: false,
	}

	creds, err := p.execCredential()
	if err != nil {
		t.Fatalf("unexpected err=%v", err)
	}

	if diff := cmp.Diff(wantStatus, creds.Status); diff != "" {
		t.Errorf("execCredential() returned unexpected diff (-want +got): %s", diff)
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
        "cluster": "user-cluster",
        "use_application_default_credentials": "false"
      },
      "core": {
        "account": "user@company.com",
        "disable_usage_reporting": "False",
        "project": "user-gke-dev"
      }
    }
  },
  "credential": {
    "access_token": "ya29.gcloud_token",
    "token_expiry": "2021-12-21T01:07:51Z"
  },
  "sentinels": {
    "config_sentinel": "/usr/local/google/home/user/.config/gcloud/config_sentinel"
  }
}`
	return []byte(fakeOutput), nil
}

func fakeK8sStartingConfig() (*clientcmdapi.Config, error) {
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
