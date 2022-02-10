package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauth "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
)

func TestGcloudPlugin(t *testing.T) {
	newYears := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

	tcs := []struct {
		name   string
		config string

		wantErr    bool
		wantStatus *clientauth.ExecCredentialStatus
	}{
		{
			name: "good",
			config: `
{
  "credential": {
    "access_token": "ya29.t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z"
  }
}
`,
			wantStatus: &clientauth.ExecCredentialStatus{
				Token:               "ya29.t0k3n",
				ExpirationTimestamp: &metav1.Time{Time: newYears},
			},
		},
		{
			name: "all good with details",
			config: `
{
  "configuration": {
    "active_configuration": "default",
    "properties": {
      "compute": {
        "region": "hoth-echobase1",
        "zone": "hoth-echobase1-c"
      },
      "core": {
        "account": "chewbacca@millenium.falcon",
        "disable_usage_reporting": "True",
        "project": "the-resistance"
      }
    }
  },
  "credential": {
    "access_token": "ya29.t0k3n",
    "token_expiry": "2022-01-01T00:00:00Z"
  }
}
`,
			wantStatus: &clientauth.ExecCredentialStatus{
				Token:               "ya29.t0k3n",
				ExpirationTimestamp: &metav1.Time{Time: newYears},
			},
		},
		{
			name:    "empty response from gcloud",
			config:  "",
			wantErr: true,
		},
		{
			name:    "response with empty json object from gcloud",
			config:  "{}",
			wantErr: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			p := &plugin{
				readGcloudConfigRaw: func() ([]byte, error) {
					return []byte(tc.config), nil
				},
			}

			creds, err := p.execCredential()
			if tc.wantErr && err == nil {
				t.Fatalf("p.execCredential() returned nil error, wanted non-nil")
			} else if tc.wantErr && err != nil {
				return
			} else if !tc.wantErr && err != nil {
				t.Fatalf("p.execCredential() returned non-nil error, want nil error: %v", err)
			}

			if diff := cmp.Diff(tc.wantStatus, creds.Status); diff != "" {
				t.Errorf("execCredential() returned unexpected diff (-want +got): %s", diff)
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
		readGcloudConfigRaw: func() ([]byte, error) {
			return []byte(gcloudConfig), nil
		},
	}

	creds, err := p.execCredential()
	if err != nil {
		t.Fatalf("unexpected err=%v", err)
	}

	if diff := cmp.Diff(wantStatus, creds.Status); diff != "" {
		t.Errorf("execCredential() returned unexpected diff (-want +got): %s", diff)
	}
}
