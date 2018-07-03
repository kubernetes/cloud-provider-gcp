package main

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/client-go/rest"
)

func TestKubeEnvToConfig(t *testing.T) {
	tests := []struct {
		desc    string
		kubeEnv string
		wantErr bool
	}{
		{
			desc: "success",
			kubeEnv: `FOO: bar
TPM_BOOTSTRAP_CERT: ZmFrZV9jZXJ0Cg==
TPM_BOOTSTRAP_KEY: ZmFrZV9rZXkK
BAZ: qux

  indented line
KUBERNETES_MASTER_NAME: 1.2.3.4`,
		},
		{
			desc: "no cert",
			kubeEnv: `TPM_BOOTSTRAP_KEY: ZmFrZV9rZXkK
KUBERNETES_MASTER_NAME: 1.2.3.4`,
			wantErr: true,
		},
		{
			desc: "no key",
			kubeEnv: `TPM_BOOTSTRAP_CERT: ZmFrZV9jZXJ0Cg==
KUBERNETES_MASTER_NAME: 1.2.3.4`,
			wantErr: true,
		},
		{
			desc: "no master",
			kubeEnv: `TPM_BOOTSTRAP_CERT: ZmFrZV9jZXJ0Cg==
TPM_BOOTSTRAP_KEY: ZmFrZV9rZXkK`,
			wantErr: true,
		},
		{
			desc:    "empty",
			wantErr: true,
		},
	}
	wantConfig := &rest.Config{
		Host: "https://1.2.3.4",
		TLSClientConfig: rest.TLSClientConfig{
			CertData: []byte("fake_cert\n"),
			KeyData:  []byte("fake_key\n"),
			CAFile:   caFilePath,
		},
		Timeout: 5 * time.Minute,
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := kubeEnvToConfig(tt.kubeEnv)
			switch {
			case err == nil && tt.wantErr:
				t.Fatal("got nil error, want non-nil")
			case err != nil && !tt.wantErr:
				t.Fatalf("got error %v, want nil", err)
			case err == nil:
				if !reflect.DeepEqual(got, wantConfig) {
					t.Errorf("got config %+v\nwant config %+v", got, wantConfig)
				}
			}
		})
	}
}
