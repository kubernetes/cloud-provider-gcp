package cred

import (
	"context"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthv1b1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type mockTokenSource struct{}

func (*mockTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		TokenType:   "bearer",
		AccessToken: "default_access_token",
		Expiry:      time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
	}, nil
}

var (
	activeUserConfig  = "default"
	fakeConfigDefault = `[core]\naccount = username@google.com\nproject = username-gke-dev\n\n[container]\nuse_application_default_credentials = false\ncluster = username-cluster\n\n[compute]\nzone = us-central1-c\n\n`

	baseCacheFile = `
{
    "current_context": "%s",
    "access_token": "%s",
    "token_expiry": "%s",
	"provider_context": "%s"
}
`
	invalidCacheFile   = "invalid_cache_file"
	fakeCurrentContext = "gke_user-gke-dev_us-east1-b_cluster-1"
	cachedAccessToken  = "ya29.cached_token"

	validCacheFile = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"Gcloud")

	cacheFileWithTokenExpired = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"Gcloud")

	cachedFileAccessTokenIsEmpty = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		"",
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"Gcloud")

	cachedFileExpiryTimestampIsMalformed = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		"bad time stamp",
		"Gcloud")

	cachedFileClusterContextChanged = fmt.Sprintf(baseCacheFile,
		"old cluster context",
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"Gcloud")

	cachedFileAuthContextChanged = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"new_provider_context")

	baseCacheFileWithAuthzToken = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "iam-ya29.gcloud_t0k3n^authz-t0k3n",
    "token_expiry": "2023-01-01T00:00:00Z",
    "provider_context": "Gcloud"
}`
	cachedFileAutzTokenPathChanged = fmt.Sprintf(baseCacheFileWithAuthzToken, "old_Path", "")
	cachedFileAutzTokenChanged     = fmt.Sprintf(baseCacheFileWithAuthzToken, "auth-token-test-file", "old-authz-t0k3n")

	wantCacheFile = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "ya29.gcloud_t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z",
    "provider_context": "Gcloud"
}`

	wantCacheFileWithAuthzToken = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "iam-ya29.gcloud_t0k3n^authz-t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z",
    "provider_context": "Gcloud"
}`

// Edge cloud test helpers
	fakeEdgeCloudLocation		= "us-central-fake"
	fakeEdgeCloudCluster 		= "fake-edge-cloud-cluster"
	fakeEdgeCloudCurrentContext = "static-edge-cloud-current-context"
	fakeEdgeCloudAuthContext	= fmt.Sprintf("GcloudEdgeCloud_%s_%s", fakeEdgeCloudLocation, fakeEdgeCloudCluster)

	validEdgeCloudCacheFile 	= fmt.Sprintf(baseCacheFile,
		fakeEdgeCloudCurrentContext,
		"EdgeCloud_CachedAccessToken",
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		fakeEdgeCloudAuthContext,
	)

	wantEdgeCloudCacheFile = fmt.Sprintf(`{
    "current_context": "%s",
    "access_token": "EdgeCloud_NewAccessToken",
    "token_expiry": "2022-01-01T00:00:00Z",
    "provider_context": "%s"
}`, fakeEdgeCloudCurrentContext, fakeEdgeCloudAuthContext)
)

func TestExecCredential(t *testing.T) {
	newYears := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	cacheFilesWrittenBySubtest := make(map[string]string)

	testCases := []struct {
		testName      string
		p             *plugin
		wantToken     *clientauthv1b1.ExecCredential
		wantCacheFile string
	}{
		{
			testName: "ApplicationDefaultCredentialsSetToTrue",
			p: &plugin{
				k8sStartingConfig: nil, // this code should be unreachable when ADC is set to true,
				getCacheFilePath:  nil, // this code should be unreachable when ADC is set to true,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				writeCacheFile:    nil, // this code should be unreachable when ADC is set to true
				tokenProvider:     &defaultCredentialsTokenProvider{
					googleDefaultTokenSource: fakeDefaultTokenSource,
				},
			},
			wantToken: fakeExecCredential("default_access_token", &metav1.Time{Time: newYears}),
		},
		{
			testName: "NewGcloudAccessToken",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "GetK8sStartingConfigFails",
			p: &plugin{
				k8sStartingConfig: func() (*clientcmdapi.Config, error) {
					return nil, fmt.Errorf("error reading starting config")
				},
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken: fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
		},
		{
			testName: "CachedFileIsValid",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(validCacheFile), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:      &gcloudTokenProvider{
					readGcloudConfigRaw: nil, // Code should be unreachable in this test
					readFile:            nil, // Code should be unreachable in this test
				},
			},
			wantToken: fakeExecCredential("ya29.cached_token", &metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)}),
		},
		{
			testName: "cachedFileIsNotPresent",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(""), fmt.Errorf("file not found")
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileIsMalformed",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte("cache_file_is_malformed"), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileExpiryTimestampIsMalformed",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileExpiryTimestampIsMalformed), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileAccessTokenIsEmpty",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileAccessTokenIsEmpty), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileClusterContextChanged",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileClusterContextChanged), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileAuthContextChanged",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileAuthContextChanged), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:      &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "CachingFailsSafely",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				writeCacheFile:    func(content string) error { return fmt.Errorf("error writing cache file") },
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: "",
		},
		{
			testName: "GcloudAccessTokenWithAuthorizationToken",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigWithAuthzTokenOutput,
					readFile: func(filename string) ([]byte, error) {
						switch filename {
						case "/usr/local/google/home/username/.config/gcloud/active_config":
							return []byte("username-inspect-k8s"), nil
						case "/Users/username/.config/gcloud/active_config":
							return []byte("username-inspect-k8s"), nil
						case "/usr/local/google/home/username/.config/gcloud/configurations/fakeConfigDefault":
							return []byte(""), nil
						case "auth-token-test-file":
							return []byte("authz-t0k3n"), nil
						default:
							return fakeReadFile(filename)
						}
					},
				},
			},
			wantToken:     fakeExecCredential("iam-ya29.gcloud_t0k3n^authz-t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFileWithAuthzToken,
		},
		{
			testName: "CachedFileWithAuthzTokenFilePathChanged",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileAutzTokenPathChanged), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigWithAuthzTokenOutput,
					readFile: func(filename string) ([]byte, error) {
						switch filename {
						case "auth-token-test-file":
							return []byte("authz-t0k3n"), nil
						default:
							return fakeReadFile(filename)
						}
					},
				},
			},
			wantToken:     fakeExecCredential("iam-ya29.gcloud_t0k3n^authz-t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFileWithAuthzToken,
		},
		{
			testName: "CachedFileWithAuthzTokenChanged",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileAutzTokenChanged), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigWithAuthzTokenOutput,
					readFile: func(filename string) ([]byte, error) {
						switch filename {
						case "auth-token-test-file":
							return []byte("authz-t0k3n"), nil
						default:
							return fakeReadFile(filename)
						}
					},
				},
			},
			wantToken:     fakeExecCredential("iam-ya29.gcloud_t0k3n^authz-t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFileWithAuthzToken,
		},
		{
			testName: "EdgeCloudNewAccessToken",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfigEdgeCloud,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudEdgeCloudTokenProvider{
					location:    fakeEdgeCloudLocation,
					clusterName: fakeEdgeCloudCluster,
					getTokenRaw: fakeEdgeCloudTokenOutput,
				},
			},
			wantToken:     fakeExecCredential("EdgeCloud_NewAccessToken", &metav1.Time{Time: newYears}),
			wantCacheFile: wantEdgeCloudCacheFile,
		},
		{
			testName: "EdgeCloudCacheFileIsValid",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfigEdgeCloud,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(validEdgeCloudCacheFile), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudEdgeCloudTokenProvider{
					location:    fakeEdgeCloudLocation,
					clusterName: fakeEdgeCloudCluster,
					getTokenRaw: nil, // Code should be unreachable in this test
				},
			},
			wantToken: fakeExecCredential("EdgeCloud_CachedAccessToken", &metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)}),
		},
		{
			testName: "EdgeCloudCacheFileClusterLocationChanged",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfigEdgeCloud,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(validEdgeCloudCacheFile), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:           fakeTimeNow,
				tokenProvider:     &gcloudEdgeCloudTokenProvider{
					location:    "different-location",
					clusterName: fakeEdgeCloudCluster,
					getTokenRaw: fakeEdgeCloudTokenOutput,
				},
			},
			wantToken:     fakeExecCredential("EdgeCloud_NewAccessToken", &metav1.Time{Time: newYears}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			// Setup cache file writer func for subtest
			tc.p.writeCacheFile = func(content string) error {
				cacheFilesWrittenBySubtest[tc.testName] = content
				return nil
			}

			// Run
			ec, err := tc.p.execCredential()
			if err != nil {
				t.Fatalf("err should be nil")
			}

			if diff := cmp.Diff(tc.wantToken, ec); diff != "" {
				t.Errorf("execCredential() returned unexpected diff (-want +got): %s", diff)
			}

			if tc.wantCacheFile != "" {
				gotCacheFile, present := cacheFilesWrittenBySubtest[tc.testName]
				if !present {
					t.Fatalf("Cache file is expected for subtest %s", tc.testName)
				}
				if diff := cmp.Diff(tc.wantCacheFile, gotCacheFile); diff != "" {
					t.Errorf("unexpected cachefile write (-want +got): %s", diff)
				}
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
    "access_token": "ya29.gcloud_t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z"
  },
  "sentinels": {
    "config_sentinel": "/usr/local/google/home/user/.config/gcloud/config_sentinel"
  }
}`
	return []byte(fakeOutput), nil
}

func fakeGcloudConfigWithAuthzTokenOutput() ([]byte, error) {
	return []byte(`
{
  "configuration": {
    "active_configuration": "inspect-username-k8s",
    "properties": {
      "auth": {
        "authorization_token_file": "auth-token-test-file"
      }
    }
  },
  "credential": {
    "access_token": "ya29.gcloud_t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z"
  }
}
`), nil
}

func fakeEdgeCloudTokenOutput(location string, clusterName string) ([]byte, error) {
	return []byte(`
	{
		"accessToken": "EdgeCloud_NewAccessToken",
		"expireTime": "2022-01-01T00:00:00Z"
	  }
`), nil
}

func fakeReadFile(filename string) ([]byte, error) {
	m := make(map[string]string)

	m[path.Join("/home/username/.kube", cacheFileName)] = cacheFileWithTokenExpired
	m[path.Join("/Users/username/.config/gcloud", activeConfig)] = activeUserConfig
	m["/Users/username/.config/gcloud/configurations/fakeConfigDefault"] = fakeConfigDefault

	file, present := m[filename]
	if !present {
		return []byte(""), fmt.Errorf("filename %s was not found", filename)
	}
	return []byte(file), nil
}

func fakeGetCacheFilePath() string {
	return path.Join("/home/username/.kube", cacheFileName)
}

func fakeK8sStartingConfig() (*clientcmdapi.Config, error) {
	return &clientcmdapi.Config{
		Kind:        "Config",
		APIVersion:  "v1",
		Preferences: clientcmdapi.Preferences{},
		Clusters:    nil,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"gke_user-gke-dev_us-east1-b_cluster-1": {
				Exec: &clientcmdapi.ExecConfig{
					Env: nil,
				},
			},
			fakeEdgeCloudCurrentContext: {
				Exec: &clientcmdapi.ExecConfig{
					Env: nil,
				},
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"gke_user-gke-dev_us-east1-b_cluster-1": {
				Cluster:  "gke_user-gke-dev_us-east1-b_cluster-1",
				AuthInfo: "gke_user-gke-dev_us-east1-b_cluster-1",
			},
			fakeEdgeCloudCurrentContext: {
				Cluster:  fakeEdgeCloudCurrentContext,
				AuthInfo: fakeEdgeCloudCurrentContext,
			},
		},
		CurrentContext: "gke_user-gke-dev_us-east1-b_cluster-1",
		Extensions:     nil,
	}, nil
}

func fakeK8sStartingConfigEdgeCloud() (*clientcmdapi.Config, error) {
	config, err := fakeK8sStartingConfig()
	config.CurrentContext = fakeEdgeCloudCurrentContext

	return config, err
}

func fakeExecCredential(token string, expiry *metav1.Time) *clientauthv1b1.ExecCredential {
	return &clientauthv1b1.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ExecCredential",
			APIVersion: "client.authentication.k8s.io/v1beta1",
		},
		Status: &clientauthv1b1.ExecCredentialStatus{
			Token:               token,
			ExpirationTimestamp: expiry,
		},
	}
}

func fakeTimeNow() time.Time {
	return time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)
}
