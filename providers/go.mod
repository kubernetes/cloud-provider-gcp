module k8s.io/cloud-provider-gcp/providers

go 1.16

require (
	cloud.google.com/go v0.54.0
	github.com/GoogleCloudPlatform/k8s-cloud-provider v0.0.0-20200415212048-7901bc822317
	github.com/google/go-cmp v0.5.2
	github.com/stretchr/testify v1.6.1
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/api v0.46.0
	gopkg.in/gcfg.v1 v1.2.0
	gopkg.in/warnings.v0 v0.1.1 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/cloud-provider v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)

replace (
	cloud.google.com/go => cloud.google.com/go v0.54.0
	github.com/GoogleCloudPlatform/k8s-cloud-provider => github.com/GoogleCloudPlatform/k8s-cloud-provider v0.0.0-20200415212048-7901bc822317
	github.com/google/go-cmp => github.com/google/go-cmp v0.5.2
	github.com/stretchr/testify => github.com/stretchr/testify v1.6.1
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/api => google.golang.org/api v0.46.0
	gopkg.in/gcfg.v1 => gopkg.in/gcfg.v1 v1.2.0
	gopkg.in/warnings.v0 => gopkg.in/warnings.v0 v0.1.1 // indirect
	k8s.io/api => k8s.io/api v0.22.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.0
	k8s.io/client-go => k8s.io/client-go v0.22.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.0
	k8s.io/component-base => k8s.io/component-base v0.22.0
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.8.0
	k8s.io/utils => k8s.io/utils v0.0.0-20210802155522-efc7438f0176 // indirect
)
