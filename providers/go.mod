module k8s.io/cloud-provider-gcp/providers

go 1.15

require (
	cloud.google.com/go v0.65.0
	github.com/GoogleCloudPlatform/k8s-cloud-provider v0.0.0-20200415212048-7901bc822317
	github.com/google/go-cmp v0.5.2
	github.com/stretchr/testify v1.6.1
	golang.org/x/oauth2 v0.0.0-20210112200429-01de73cf58bd
	google.golang.org/api v0.30.0
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v0.20.0
	k8s.io/cloud-provider v0.20.0
	k8s.io/component-base v0.20.0
	k8s.io/klog/v2 v2.4.0
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)

replace (
	k8s.io/api => k8s.io/api v0.20.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.0
	k8s.io/client-go => k8s.io/client-go v0.20.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.20.0
	k8s.io/component-base => k8s.io/component-base v0.20.0
)
