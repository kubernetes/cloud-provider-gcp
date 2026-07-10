package main

import (
	"context"
	"crypto/sha256"
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
	activeUserConfig              = "default"
	fakeConfigDefault             = `[core]\naccount = username@google.com\nproject = username-gke-dev\n\n[container]\nuse_application_default_credentials = false\ncluster = username-cluster\n\n[compute]\nzone = us-central1-c\n\n`
	fakeAccount                   = "fakeAccount"
	fakeProject                   = "fakeProject"
	fakeImpersonateServiceAccount = "fakeServiceAccount"

	baseCacheFile = `
{
    "current_context": "%s",
    "access_token": "%s",
    "token_expiry": "%s",
	"extra_args": "%s",
    "gcloud_config": {
        "config_dir": "%s",
        "active_config": "%s",
        "config_hash": "%s"
    }
}
`
	invalidCacheFile   = "invalid_cache_file"
	fakeCurrentContext = "gke_user-gke-dev_us-east1-b_cluster-1"
	cachedAccessToken  = "ya29.cached_token"
	fakeCloudSDKConfig = "/Users/username/.config/gcloud"
	fakeConfigName     = "default"
	fakeConfigContents = `[core]
account = username@google.com
project = username-gke-dev

[container]
use_application_default_credentials = false
cluster = username-cluster

[compute]
zone = us-central1-c
`
	fakeChangedConfigContents = `[core]
account = other-user@google.com
project = username-gke-dev

[container]
use_application_default_credentials = false
cluster = username-cluster

[compute]
zone = us-central1-c
`
	fakeConfigHash        = fmt.Sprintf("%x", sha256.Sum256([]byte(fakeConfigContents)))
	fakeChangedConfigHash = fmt.Sprintf("%x", sha256.Sum256([]byte(fakeChangedConfigContents)))

	validCacheFile = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"",
		fakeCloudSDKConfig,
		fakeConfigName,
		fakeConfigHash)

	validCacheFileWithExtraArgs = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		fmt.Sprintf("--project=%s --account=%s", fakeProject, fakeAccount),
		fakeCloudSDKConfig,
		fakeConfigName,
		fakeConfigHash)

	validCacheFileWithImpersonateServiceAccount = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"--impersonate-service-account="+fakeImpersonateServiceAccount,
		fakeCloudSDKConfig,
		fakeConfigName,
		fakeConfigHash)

	cacheFileWithTokenExpired = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"",
		fakeCloudSDKConfig,
		fakeConfigName,
		fakeConfigHash)

	cachedFileAccessTokenIsEmpty = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		"",
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"",
		fakeCloudSDKConfig,
		fakeConfigName,
		fakeConfigHash)

	cachedFileExpiryTimestampIsMalformed = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		"bad time stamp",
		"",
		fakeCloudSDKConfig,
		fakeConfigName,
		fakeConfigHash)

	cachedFileClusterContextChanged = fmt.Sprintf(baseCacheFile,
		"old cluster context",
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"",
		fakeCloudSDKConfig,
		fakeConfigName,
		fakeConfigHash)

	cachedFileWithBadExtraArgs = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"FakeArgs",
		fakeCloudSDKConfig,
		fakeConfigName,
		fakeConfigHash)

	cachedFileWithOldGcloudConfig = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"",
		fakeCloudSDKConfig,
		fakeConfigName,
		fakeChangedConfigHash)

	baseCacheFileWithAuthzToken = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "ya29.gcloud_t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z",
    "extra_args": ""
}`
	cachedFileAutzTokenPathChanged = baseCacheFileWithAuthzToken //fmt.Sprintf(baseCacheFileWithAuthzToken, "old_Path", "")
	cachedFileAutzTokenChanged     = baseCacheFileWithAuthzToken //fmt.Sprintf(baseCacheFileWithAuthzToken, "auth-token-test-file", "old-authz-t0k3n")

	wantCacheFile = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "ya29.gcloud_t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z",
    "extra_args": "",
    "gcloud_config": {
        "config_dir": "/Users/username/.config/gcloud",
        "active_config": "default",
        "config_hash": "` + fakeConfigHash + `"
    }
}`

	wantCacheFileWithExtraArgs = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "ya29.gcloud_t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z",
    "extra_args": "--project=` + fakeProject + ` --account=` + fakeAccount + `",
    "gcloud_config": {
        "config_dir": "/Users/username/.config/gcloud",
        "active_config": "default",
        "config_hash": "` + fakeConfigHash + `"
    }
}`

	wantCacheFileImpersonateServiceAccount = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "ya29.gcloud_t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z",
    "extra_args": "--impersonate-service-account=` + fakeImpersonateServiceAccount + `",
    "gcloud_config": {
        "config_dir": "/Users/username/.config/gcloud",
        "active_config": "default",
        "config_hash": "` + fakeConfigHash + `"
    }
}`

	wantCacheFileWithAuthzToken = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "ya29.gcloud_t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z",
    "extra_args": "",
    "gcloud_config": {
        "config_dir": "/Users/username/.config/gcloud",
        "active_config": "default",
        "config_hash": "` + fakeConfigHash + `"
    }
}`

	// Edge cloud test helpers
	fakeEdgeCloudLocation = "us-central-fake"
	fakeEdgeCloudCluster  = "fake-edge-cloud-cluster"
	kubeCtlStartingConfig = fakeCurrentContext

	validEdgeCloudCacheFile = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "EdgeCloud_CachedAccessToken",
    "token_expiry": "2022-01-03T00:00:00Z",
    "extra_args": "fake-edge-cloud-cluster --project= --location=us-central-fake --format=json"
}`

	expiredEdgeCloudCacheFile = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "EdgeCloud_CachedAccessToken",
    "token_expiry": "2022-01-01T00:00:00Z",
    "extra_args": "fake-edge-cloud-cluster --project= --location=us-central-fake --format=json"
}`

	wantEdgeCloudCacheFile = fmt.Sprintf(`{
    "current_context": "%s",
    "access_token": "EdgeCloud_NewAccessToken",
    "token_expiry": "2022-01-01T00:00:00Z",
    "extra_args": "fake-edge-cloud-cluster --project=%s --location=us-central-fake --format=json"
}`, kubeCtlStartingConfig, fakeProject)
)

func TestExecCredential(t *testing.T) {
	newYears := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	cacheFilesWrittenBySubtest := make(map[string]string)
	t.Setenv("CLOUDSDK_CONFIG", fakeCloudSDKConfig)
	t.Setenv(strictCacheInvalidationEnvVar, "true")

	testCases := []struct {
		testName       string
		p              *plugin
		wantToken      *clientauthv1b1.ExecCredential
		wantCacheFile  string
		wantGcloudArgs []string
	}{
		{
			testName: "ApplicationDefaultCredentialsSetToTrue",
			p: &plugin{
				k8sStartingConfig: nil, // this code should be unreachable when ADC is set to true,
				getCacheFilePath:  nil, // this code should be unreachable when ADC is set to true,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				writeCacheFile:    nil, // this code should be unreachable when ADC is set to true
				tokenProvider: &defaultCredentialsTokenProvider{
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
				tokenProvider: &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
			wantGcloudArgs: []string{
				"config",
				"config-helper",
				"--format=json",
			},
		},
		{
			testName: "NewGcloudAccessTokenWithExtraArgs",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
					account:             fakeAccount,
					project:             fakeProject,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFileWithExtraArgs,
			wantGcloudArgs: []string{
				"config",
				"config-helper",
				"--format=json",
				"--project=" + fakeProject,
				"--account=" + fakeAccount,
			},
		},
		{
			testName: "NewGcloudAccessTokenWithImpersonateServiceAccount",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
					readGcloudConfigRaw:       fakeGcloudConfigOutput,
					readFile:                  fakeReadFile,
					impersonateServiceAccount: fakeImpersonateServiceAccount,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFileImpersonateServiceAccount,
			wantGcloudArgs: []string{
				"config",
				"config-helper",
				"--format=json",
				"--impersonate-service-account=" + fakeImpersonateServiceAccount,
			},
		},
		{
			testName: "GetK8sStartingConfigFails",
			p: &plugin{
				k8sStartingConfig: func() (*clientcmdapi.Config, error) {
					return nil, fmt.Errorf("error reading starting config")
				},
				getCacheFilePath: fakeGetCacheFilePath,
				readFile:         fakeReadFile,
				timeNow:          fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
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
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
					readGcloudConfigRaw: nil, // Code should be unreachable in this test
					readFile:            nil, // Code should be unreachable in this test
				},
			},
			wantToken: fakeExecCredential("ya29.cached_token", &metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)}),
		},
		{
			testName: "CachedFileIsValidExtraArgs",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(validCacheFileWithExtraArgs), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
					readGcloudConfigRaw: nil, // Code should be unreachable in this test
					readFile:            nil, // Code should be unreachable in this test
					project:             fakeProject,
					account:             fakeAccount,
				},
			},
			wantToken: fakeExecCredential("ya29.cached_token", &metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)}),
		},
		{
			testName: "CachedFileIsValidImpersonateServiceAccount",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(validCacheFileWithImpersonateServiceAccount), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
					readGcloudConfigRaw:       nil, // Code should be unreachable in this test
					readFile:                  nil, // Code should be unreachable in this test
					impersonateServiceAccount: fakeImpersonateServiceAccount,
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
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
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
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
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
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
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
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
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
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileExtraArgsChanged",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileWithBadExtraArgs), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileGcloudConfigChanged",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileWithOldGcloudConfig), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
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
				tokenProvider: &gcloudTokenProvider{
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
				tokenProvider: &gcloudTokenProvider{
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
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFileWithAuthzToken,
		},
		{
			testName: "GcloudAccessTokenFromAccessTokenFile",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
					readGcloudConfigRaw: fakeGcloudConfigWithAccessTokenFileOutput,
					readFile:            fakeReadFile,
				},
			},
			wantToken:     fakeExecCredential("ya29.token_from_file", nil),
			wantCacheFile: "",
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
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
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
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
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
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudTokenProvider{
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
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFileWithAuthzToken,
		},
		{
			testName: "EdgeCloudExpectsCachedTokenWhenValidCacheExists",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(validEdgeCloudCacheFile), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudEdgeCloudTokenProvider{
					location:    fakeEdgeCloudLocation,
					clusterName: fakeEdgeCloudCluster,
					getTokenRaw: nil, // Code should be unreachable in this test
				},
			},
			wantToken: fakeExecCredential("EdgeCloud_CachedAccessToken", &metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)}),
		},
		{
			testName: "EdgeCloudExpectsNewTokenWhenNoCacheExists",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile:          fakeReadFile,
				timeNow:           fakeTimeNow,
				tokenProvider: &gcloudEdgeCloudTokenProvider{
					location:    fakeEdgeCloudLocation,
					clusterName: fakeEdgeCloudCluster,
					project:     fakeProject,
					getTokenRaw: fakeEdgeCloudTokenOutput,
				},
			},
			wantToken:     fakeExecCredential("EdgeCloud_NewAccessToken", &metav1.Time{Time: newYears}),
			wantCacheFile: wantEdgeCloudCacheFile,
			wantGcloudArgs: []string{
				"edge-cloud",
				"container",
				"clusters",
				"print-access-token",
				"fake-edge-cloud-cluster",
				"--project=" + fakeProject,
				"--location=us-central-fake",
				"--format=json",
			},
		},
		{
			testName: "EdgeCloudExpectsNewTokenWhenCacheFileExpires",
			p: &plugin{
				k8sStartingConfig: fakeK8sStartingConfig,
				getCacheFilePath:  fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(expiredEdgeCloudCacheFile), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow: fakeTimeNow,
				tokenProvider: &gcloudEdgeCloudTokenProvider{
					location:    fakeEdgeCloudLocation,
					clusterName: fakeEdgeCloudCluster,
					project:     fakeProject,
					getTokenRaw: fakeEdgeCloudTokenOutput,
				},
			},
			wantToken:     fakeExecCredential("EdgeCloud_NewAccessToken", &metav1.Time{Time: newYears}),
			wantCacheFile: wantEdgeCloudCacheFile,
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
				t.Fatalf("err should be nil: %v", err)
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

			if tc.wantGcloudArgs != nil {
				if diff := cmp.Diff(tc.wantGcloudArgs, tc.p.tokenProvider.getGcloudArgs()); diff != "" {
					t.Errorf("unexpected gcloud args (-want +got): %s", diff)
				}
			}
		})
	}
}

func TestCloudsdkBasedGcloudAccessToken(t *testing.T) {
	p := &plugin{
		k8sStartingConfig: nil,
		getCacheFilePath:  nil,
		readFile:          nil,
		timeNow:           nil,
		tokenProvider: &gcloudTokenProvider{
			readGcloudConfigRaw: nil,
			readFile:            nil,
		},
	}

	tokenForEnvVar := "gcloud_token_in_env_var"
	t.Setenv(cloudsdkAuthAccessEnvVar, tokenForEnvVar)

	ec, err := p.execCredential()
	if err != nil {
		t.Fatalf("err should be nil")
	}

	t.Setenv(cloudsdkAuthAccessEnvVar, "")

	if diff := cmp.Diff(ec.Status.Token, tokenForEnvVar); diff != "" {
		t.Errorf("unexpected token (-want +got): %s", diff)
	}

	if ec.Status.ExpirationTimestamp != nil {
		t.Errorf("unexpected expiration time stamp: %v", ec.Status.ExpirationTimestamp)
	}
}

func TestCachedGcloudAccessTokenFallsBackWhenConfigCannotBeRead(t *testing.T) {
	t.Setenv(strictCacheInvalidationEnvVar, "true")
	t.Setenv("CLOUDSDK_CONFIG", fakeCloudSDKConfig)
	p := &plugin{
		k8sStartingConfig: fakeK8sStartingConfig,
		getCacheFilePath:  fakeGetCacheFilePath,
		readFile: func(filename string) ([]byte, error) {
			if filename == fakeGetCacheFilePath() {
				return []byte(validCacheFile), nil
			}
			return nil, fmt.Errorf("cannot read %s", filename)
		},
		timeNow:       fakeTimeNow,
		tokenProvider: &gcloudTokenProvider{},
	}

	token, _, err := p.getCachedGcloudAccessToken()
	if err != nil {
		t.Fatalf("getCachedGcloudAccessToken() returned an error: %v", err)
	}
	if token != cachedAccessToken {
		t.Fatalf("getCachedGcloudAccessToken() token = %q, want %q", token, cachedAccessToken)
	}
}

func TestCurrentGcloudConfigHonorsActiveConfigEnvironmentVariable(t *testing.T) {
	t.Setenv("CLOUDSDK_CONFIG", fakeCloudSDKConfig)
	t.Setenv(cloudsdkActiveConfigEnvVar, "overridden")
	p := &plugin{
		readFile: func(filename string) ([]byte, error) {
			if filename != path.Join(fakeCloudSDKConfig, "configurations", "config_overridden") {
				return nil, fmt.Errorf("unexpected path %s", filename)
			}
			return []byte(fakeConfigContents), nil
		},
		tokenProvider: &gcloudTokenProvider{},
	}

	config, err := p.currentGcloudConfig()
	if err != nil {
		t.Fatalf("currentGcloudConfig() returned an error: %v", err)
	}
	if config.ActiveConfig != "overridden" {
		t.Errorf("currentGcloudConfig() active config = %q, want overridden", config.ActiveConfig)
	}
	if config.ConfigHash != fakeConfigHash {
		t.Errorf("currentGcloudConfig() config hash = %q, want %q", config.ConfigHash, fakeConfigHash)
	}
}

func fakeDefaultTokenSource(ctx context.Context, scope ...string) (oauth2.TokenSource, error) {
	return &mockTokenSource{}, nil
}

func fakeGcloudConfigOutput(extraArgs []string) ([]byte, error) {
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

func fakeGcloudConfigWithAuthzTokenOutput(extraArgs []string) ([]byte, error) {
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

func fakeGcloudConfigWithAccessTokenFileOutput(extraArgs []string) ([]byte, error) {
	return []byte(`
{
  "configuration": {
    "active_configuration": "default",
    "properties": {
      "auth": {
        "access_token_file": "/usr/local/google/home/username/Desktop/gcloud_test_token"
      },
      "compute": {
        "zone": "us-central1-c"
      },
      "container": {
        "cluster": "username-cluster",
        "use_application_default_credentials": "false"
      },
      "core": {
        "account": "username@google.com",
        "disable_usage_reporting": "False",
        "project": "username-gke-dev"
      }
    }
  },
  "credential": {
    "access_token": "ya29.token_from_file",
    "id_token": null,
    "token_expiry": null
  },
  "sentinels": {
    "config_sentinel": "/usr/local/google/home/username/.config/gcloud/config_sentinel"
  }
}
`), nil
}

func fakeReadFile(filename string) ([]byte, error) {
	m := make(map[string]string)

	m[path.Join("/home/username/.kube", cacheFileName)] = cacheFileWithTokenExpired
	m[path.Join("/Users/username/.config/gcloud", activeConfig)] = activeUserConfig
	m["/Users/username/.config/gcloud/configurations/config_default"] = fakeConfigContents
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
		},
		Contexts: map[string]*clientcmdapi.Context{
			"gke_user-gke-dev_us-east1-b_cluster-1": {
				Cluster:  "gke_user-gke-dev_us-east1-b_cluster-1",
				AuthInfo: "gke_user-gke-dev_us-east1-b_cluster-1",
			},
		},
		CurrentContext: "gke_user-gke-dev_us-east1-b_cluster-1",
		Extensions:     nil,
	}, nil
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

func fakeEdgeCloudTokenOutput(extraArgs []string) ([]byte, error) {
	return []byte(`
	{
		"accessToken": "EdgeCloud_NewAccessToken",
		"expireTime": "2022-01-01T00:00:00Z"
	  }
`), nil
}
