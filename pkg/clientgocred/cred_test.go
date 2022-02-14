package clientgocred

import (
	"context"
	"errors"
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
    "gcloud_config": {
        "user_config_directory": "%s",
        "active_user_config": "%s",
        "active_configuration_path": "%s",
        "active_configuration": "%s",
        "authorization_token_path": "",
        "authorization_token": ""
    }
}
`
	invalidCacheFile        = "invalid_cache_file"
	fakeCurrentContext      = "gke_user-gke-dev_us-east1-b_cluster-1"
	cachedAccessToken       = "ya29.cached_token"
	userConfigDirectory     = "/Users/username/.config/gcloud"
	activeConfigurationPath = "/Users/username/.config/gcloud/configurations/fakeConfigDefault"
	activeConfiguration     = `[core]\\naccount = username@google.com\\nproject = username-gke-dev\\n\\n[container]\\nuse_application_default_credentials = false\\ncluster = username-cluster\\n\\n[compute]\\nzone = us-central1-c\\n\\n`

	validCacheFile = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		userConfigDirectory,
		activeUserConfig,
		activeConfigurationPath,
		activeConfiguration)

	cacheFileWithTokenExpired = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		userConfigDirectory,
		activeUserConfig,
		activeConfigurationPath,
		activeConfiguration)

	cachedFileAccessTokenIsEmpty = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		"",
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		userConfigDirectory,
		activeUserConfig,
		activeConfigurationPath,
		activeConfiguration)

	cachedFileExpiryTimestampIsMalformed = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		"bad time stamp",
		userConfigDirectory,
		activeUserConfig,
		activeConfigurationPath,
		activeConfiguration)

	cachedFileClusterContextChanged = fmt.Sprintf(baseCacheFile,
		"old cluster context",
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		userConfigDirectory,
		activeUserConfig,
		activeConfigurationPath,
		activeConfiguration)

	cachedFileGcloudActiveDirChanged = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"old/user/config/dir",
		activeUserConfig,
		activeConfigurationPath,
		activeConfiguration)

	cachedFileGcloudActiveConfigPathChanged = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		userConfigDirectory,
		activeUserConfig,
		"old_active_config_path",
		activeConfiguration)

	cachedFileGcloudActiveConfigChanged = fmt.Sprintf(baseCacheFile,
		fakeCurrentContext,
		cachedAccessToken,
		time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		userConfigDirectory,
		activeUserConfig,
		activeConfigurationPath,
		"old active config")

	baseCacheFileWithAuthzToken = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "iam-ya29.gcloud_t0k3n^authz-t0k3n",
    "token_expiry": "2023-01-01T00:00:00Z",
    "gcloud_config": {
        "user_config_directory": "/Users/username/.config/gcloud",
        "active_user_config": "username-inspect-k8s",
        "active_configuration_path": "/Users/username/.config/gcloud/configurations/fakeConfigDefault",
        "active_configuration": "[core]\\naccount = username@google.com\\nproject = username-gke-dev\\n\\n[container]\\nuse_application_default_credentials = false\\ncluster = username-cluster\\n\\n[compute]\\nzone = us-central1-c\\n\\n",
        "authorization_token_path": "%s",
        "authorization_token": "%s"
    }
}`
	cachedFileAutzTokenPathChanged = fmt.Sprintf(baseCacheFileWithAuthzToken, "old_Path", "")
	cachedFileAutzTokenChanged     = fmt.Sprintf(baseCacheFileWithAuthzToken, "auth-token-test-file", "old-authz-t0k3n")

	wantCacheFile = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "ya29.gcloud_t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z",
    "gcloud_config": {
        "user_config_directory": "/Users/username/.config/gcloud",
        "active_user_config": "default",
        "active_configuration_path": "/Users/username/.config/gcloud/configurations/fakeConfigDefault",
        "active_configuration": "[core]\\naccount = username@google.com\\nproject = username-gke-dev\\n\\n[container]\\nuse_application_default_credentials = false\\ncluster = username-cluster\\n\\n[compute]\\nzone = us-central1-c\\n\\n",
        "authorization_token_path": "",
        "authorization_token": ""
    }
}`

	wantCacheFileWithAuthzToken = `{
    "current_context": "gke_user-gke-dev_us-east1-b_cluster-1",
    "access_token": "iam-ya29.gcloud_t0k3n^authz-t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z",
    "gcloud_config": {
        "user_config_directory": "/Users/username/.config/gcloud",
        "active_user_config": "username-inspect-k8s",
        "active_configuration_path": "/Users/username/.config/gcloud/configurations/fakeConfigDefault",
        "active_configuration": "[core]\\naccount = username@google.com\\nproject = username-gke-dev\\n\\n[container]\\nuse_application_default_credentials = false\\ncluster = username-cluster\\n\\n[compute]\\nzone = us-central1-c\\n\\n",
        "authorization_token_path": "auth-token-test-file",
        "authorization_token": "authz-t0k3n"
    }
}`
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
				googleDefaultTokenSource:         fakeDefaultTokenSource,
				readGcloudConfigRaw:              nil, // this code should be unreachable when ADC is set to true,
				readGcloudInfoRaw:                nil, // this code should be unreachable when ADC is set to true,
				k8sStartingConfig:                nil, // this code should be unreachable when ADC is set to true,
				getCacheFilePath:                 nil, // this code should be unreachable when ADC is set to true,
				readFile:                         fakeReadFile,
				timeNow:                          fakeTimeNow,
				writeCacheFile:                   nil, // this code should be unreachable when ADC is set to true
				useApplicationDefaultCredentials: true,
			},
			wantToken: fakeExecCredential("default_access_token", &metav1.Time{Time: newYears}),
		},
		{
			testName: "NewGcloudAccessToken",
			p: &plugin{
				googleDefaultTokenSource:         nil,
				readGcloudConfigRaw:              fakeGcloudConfigOutput,
				readGcloudInfoRaw:                fakeGcloudInfoOutput,
				k8sStartingConfig:                fakeK8sStartingConfig,
				getCacheFilePath:                 fakeGetCacheFilePath,
				readFile:                         fakeReadFile,
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "GetK8sStartingConfigFails",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig: func() (*clientcmdapi.Config, error) {
					return nil, fmt.Errorf("error reading starting config")
				},
				getCacheFilePath:                 fakeGetCacheFilePath,
				readFile:                         fakeReadFile,
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken: fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
		},
		{
			testName: "GcloudAccessTokenFailureFallbackToADC",
			p: &plugin{
				googleDefaultTokenSource: fakeDefaultTokenSource,
				readGcloudConfigRaw: func() ([]byte, error) {
					return []byte("bad token string"), nil
				},
				readGcloudInfoRaw:                fakeGcloudInfoOutput,
				k8sStartingConfig:                fakeK8sStartingConfig,
				getCacheFilePath:                 fakeGetCacheFilePath,
				readFile:                         fakeReadFile,
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken: fakeExecCredential("default_access_token", &metav1.Time{Time: newYears}),
		},
		{
			testName: "GcloudCommandFailureFallbackToADC",
			p: &plugin{
				googleDefaultTokenSource: fakeDefaultTokenSource,
				readGcloudConfigRaw: func() ([]byte, error) {
					return []byte("gcloud_command_failure"), errors.New("gcloud command failure")
				},
				readGcloudInfoRaw:                fakeGcloudInfoOutput,
				k8sStartingConfig:                fakeK8sStartingConfig,
				getCacheFilePath:                 fakeGetCacheFilePath,
				readFile:                         fakeReadFile,
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken: fakeExecCredential("default_access_token", &metav1.Time{Time: newYears}),
		},
		{
			testName: "CachedFileIsValid",
			p: &plugin{
				googleDefaultTokenSource: nil, // Code should be unreachable in this test
				readGcloudConfigRaw:      nil, // Code should be unreachable in this test
				readGcloudInfoRaw:        nil, // Code should be unreachable in this test
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(validCacheFile), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken: fakeExecCredential("ya29.cached_token", &metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)}),
		},
		{
			testName: "cachedFileIsNotPresent",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(""), fmt.Errorf("file not found")
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileIsMalformed",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte("cache_file_is_malformed"), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileExpiryTimestampIsMalformed",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileExpiryTimestampIsMalformed), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileAccessTokenIsEmpty",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileAccessTokenIsEmpty), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileClusterContextChanged",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileClusterContextChanged), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileGcloudActiveDirChanged",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileGcloudActiveDirChanged), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileGcloudActiveConfigPathChanged",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileGcloudActiveConfigPathChanged), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "cachedFileGcloudActiveConfigChanged",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileGcloudActiveConfigChanged), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "CachedFileGcloudActiveUserConfigChanged",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(fmt.Sprintf(
							baseCacheFile,
							"gke_user-gke-dev_us-east1-b_cluster-1",
							"ya29.cached_token",
							time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
							"/Users/username/.config/gcloud",
							"old user config",
							"/Users/username/.config/gcloud/configurations/fakeConfigDefault",
							`[core]\naccount = username@google.com\nproject = username-gke-dev\n\n[container]\nuse_application_default_credentials = false\ncluster = username-cluster\n\n[compute]\nzone = us-central1-c\n\n`)), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow: fakeTimeNow,
				writeCacheFile: func(content string) error {
					cacheFilesWrittenBySubtest["NewGcloudAccessToken"] = content
					return nil
				},
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "CachedFileGcloudActiveConfigFilePathChanged",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(fmt.Sprintf(baseCacheFile,
							"gke_user-gke-dev_us-east1-b_cluster-1", "ya29.cached_token",
							time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
							"/Users/username/.config/gcloud",
							activeConfig,
							"old_file_path",
							`[core]\naccount = username@google.com\nproject = username-gke-dev\n\n[container]\nuse_application_default_credentials = false\ncluster = username-cluster\n\n[compute]\nzone = us-central1-c\n\n`)), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow: fakeTimeNow,
				writeCacheFile: func(content string) error {
					cacheFilesWrittenBySubtest["NewGcloudAccessToken"] = content
					return nil
				},
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "CachedFileGcloudActiveConfigFileContentChanged",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(fmt.Sprintf(baseCacheFile,
							"gke_user-gke-dev_us-east1-b_cluster-1", "ya29.cached_token",
							time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
							"/Users/username/.config/gcloud",
							activeConfig,
							"/Users/username/.config/gcloud/configurations/fakeConfigDefault",
							`old_file_content`)), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow: fakeTimeNow,
				writeCacheFile: func(content string) error {
					cacheFilesWrittenBySubtest["NewGcloudAccessToken"] = content
					return nil
				},
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "CachedFileGcloudActiveConfigChanged",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(fmt.Sprintf(baseCacheFile, "gke_user-gke-dev_us-east1-b_cluster-1", "ya29.cached_token", time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), "/Users/username/.config/gcloud", activeConfig, "default config changed", `[core]\naccount = username@google.com\nproject = username-gke-dev\n\n[container]\nuse_application_default_credentials = false\ncluster = username-cluster\n\n[compute]\nzone = us-central1-c\n\n`)), nil
					default:
						return fakeReadFile(filename)
					}
				},
				timeNow: fakeTimeNow,
				writeCacheFile: func(content string) error {
					cacheFilesWrittenBySubtest["NewGcloudAccessToken"] = content
					return nil
				},
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFile,
		},
		{
			testName: "CachingFailsSafely",
			p: &plugin{
				googleDefaultTokenSource:         nil,
				readGcloudConfigRaw:              fakeGcloudConfigOutput,
				readGcloudInfoRaw:                fakeGcloudInfoOutput,
				k8sStartingConfig:                fakeK8sStartingConfig,
				getCacheFilePath:                 fakeGetCacheFilePath,
				readFile:                         fakeReadFile,
				timeNow:                          fakeTimeNow,
				writeCacheFile:                   func(content string) error { return fmt.Errorf("error writing cache file") },
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("ya29.gcloud_t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: "",
		},
		{
			testName: "GcloudAccessTokenWithAuthorizationToken",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigWithAuthzTokenOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
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
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("iam-ya29.gcloud_t0k3n^authz-t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFileWithAuthzToken,
		},
		{
			testName: "CachedFileWithAuthzTokenFilePathChanged",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigWithAuthzTokenOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileAutzTokenPathChanged), nil
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
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("iam-ya29.gcloud_t0k3n^authz-t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFileWithAuthzToken,
		},
		{
			testName: "CachedFileWithAuthzTokenChanged",
			p: &plugin{
				googleDefaultTokenSource: nil,
				readGcloudConfigRaw:      fakeGcloudConfigWithAuthzTokenOutput,
				readGcloudInfoRaw:        fakeGcloudInfoOutput,
				k8sStartingConfig:        fakeK8sStartingConfig,
				getCacheFilePath:         fakeGetCacheFilePath,
				readFile: func(filename string) ([]byte, error) {
					switch filename {
					case fakeGetCacheFilePath():
						return []byte(cachedFileAutzTokenChanged), nil
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
				timeNow:                          fakeTimeNow,
				useApplicationDefaultCredentials: false,
			},
			wantToken:     fakeExecCredential("iam-ya29.gcloud_t0k3n^authz-t0k3n", &metav1.Time{Time: newYears}),
			wantCacheFile: wantCacheFileWithAuthzToken,
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

func fakeGcloudInfoOutput() ([]byte, error) {
	fakeOutput := `Google Cloud SDK [371.0.0]

Platform: [Mac OS X, x86_64] uname_result(system='Darwin', node='username-macbookpro.roam.corp.google.com', release='21.3.0', version='Darwin Kernel Version 21.3.0: Wed Jan  5 21:37:58 PST 2022; root:xnu-8019.80.24~20/RELEASE_X86_64', machine='x86_64', processor='i386')
Locale: ('en_US', 'UTF-8')
Python Version: [3.8.2 (v3.8.2:7b3ab5921f, Feb 24 2020, 17:52:18)  [Clang 6.0 (clang-600.0.57)]]
Python Location: [/Library/Frameworks/Python.framework/Versions/3.8/bin/python3]
OpenSSL: [OpenSSL 1.1.1d  10 Sep 2019]
Requests Version: [2.22.0]
urllib3 Version: [1.25.9]
Site Packages: [Disabled]

Installation Root: [/Users/username/google-cloud-sdk]
Installed Components:
  gsutil: [5.6]
  core: [2022.01.28]
  bq: [2.0.73]
  kubectl: [1.21.9]
  beta: [2022.01.28]
System PATH: [/Library/Frameworks/Python.framework/Versions/3.8/bin:/Users/username/google-cloud-sdk/bin:/usr/local/git/current/bin:/usr/local/bin:/usr/bin:/bin:/usr/local/sbin:/usr/sbin:/sbin:/usr/local/go/bin]
Python PATH: [/Users/username/google-cloud-sdk/lib/third_party:/Users/username/google-cloud-sdk/lib:/Library/Frameworks/Python.framework/Versions/3.8/lib/python38.zip:/Library/Frameworks/Python.framework/Versions/3.8/lib/python3.8:/Library/Frameworks/Python.framework/Versions/3.8/lib/python3.8/lib-dynload]
Cloud SDK on PATH: [True]
Kubectl on PATH: [/Users/username/google-cloud-sdk/bin/kubectl]

Installation Properties: [/Users/username/google-cloud-sdk/properties]
User Config Directory: [/Users/username/.config/gcloud]
Active Configuration Name: [default]
Active Configuration Path: [/Users/username/.config/gcloud/configurations/fakeConfigDefault]

Account: [username@google.com]
Project: [username-gke-dev]

Current Properties:
  [core]
    account: [username@google.com]
    disable_usage_reporting: [False]
    project: [username-gke-dev]

Logs Directory: [/Users/username/.config/gcloud/logs]
Last Log File: [/Users/username/.config/gcloud/logs/2022.02.11/13.05.17.444503.log]

git: [git version 2.35.1.265.g69c8d7142f-goog]
ssh: [OpenSSH_8.8p1, OpenSSL 1.1.1m  14 Dec 2021]




Updates are available for some Cloud SDK components.  To install them,
please run:
  $ gcloud components update
`
	return []byte(fakeOutput), nil
}

func fakeReadFile(filename string) ([]byte, error) {
	m := make(map[string]string)

	m[path.Join("/home/username/.kube", cacheFileName)] = cacheFileWithTokenExpired
	m[path.Join("/Users/username/.config/gcloud", activeConfig)] = activeUserConfig
	m["/Users/username/.config/gcloud/configurations/fakeConfigDefault"] = fakeConfigDefault

	if file, present := m[filename]; !present {
		return []byte(""), fmt.Errorf("filename %s was not found", filename)
	} else {
		return []byte(file), nil
	}
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
