package utils

import (
	"testing"

	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsSupervisor(t *testing.T) {
	tests := []struct {
		name           string
		providerConfig *v1.ProviderConfig
		want           bool
	}{
		{
			name: "Supervisor Config",
			providerConfig: &v1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						accessLevelLabelKey: "supervisor",
					},
				},
			},
			want: true,
		},
		{
			name: "Tenant Config",
			providerConfig: &v1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						accessLevelLabelKey: "tenant",
					},
				},
			},
			want: false,
		},
		{
			name: "No Label",
			providerConfig: &v1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSupervisor(tc.providerConfig)
			assert.Equal(t, tc.want, got)
		})
	}
}
