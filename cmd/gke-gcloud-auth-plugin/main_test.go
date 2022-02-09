package main

import (
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
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			p := &plugin{
				readGcloudConfigRaw: func() ([]byte, error) {
					return []byte(tc.config), nil
				},
			}

			creds, err := p.execCredential()
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v, err=%v", tc.wantErr, err)
			}
			if tc.wantErr {
				t.Log(err)
				return
			}

			if diff := cmp.Diff(tc.wantStatus, creds.Status); diff != "" {
				t.Errorf("execCredential() returned unexpected diff (-want +got): %s", diff)
			}
		})
	}
}
