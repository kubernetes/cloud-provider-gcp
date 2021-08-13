module k8s.io/cloud-provider-gcp

go 1.16

require (
	cloud.google.com/go v0.75.0
	github.com/Microsoft/hcsshim v0.8.10-0.20200715222032-5eafd1556990 // indirect
	github.com/auth0/go-jwt-middleware v1.0.1 // indirect
	github.com/container-storage-interface/spec v1.5.0 // indirect
	github.com/containernetworking/cni v0.8.1 // indirect
	github.com/coredns/corefile-migration v1.0.12 // indirect
	github.com/gofrs/flock v0.7.1
	github.com/google/cadvisor v0.39.2 // indirect
	github.com/google/go-cmp v0.5.5
	github.com/google/go-tpm v0.2.0
	github.com/google/uuid v1.1.4 // indirect
	github.com/heketi/heketi v10.3.0+incompatible // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/ishidawataru/sctp v0.0.0-20190723014705-7c296d48a2b5 // indirect
	github.com/libopenstorage/openstorage v1.0.0 // indirect
	github.com/moby/ipvs v1.0.1 // indirect
	github.com/mvdan/xurls v1.1.0 // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.3 // indirect
	github.com/opencontainers/runc v1.0.1 // indirect
	github.com/prometheus/client_golang v1.11.0
	github.com/quobyte/api v0.1.8 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/storageos/go-api v2.2.0+incompatible // indirect
	github.com/stretchr/testify v1.7.0
	golang.org/x/exp v0.0.0-20210220032938-85be41e4509f // indirect
	golang.org/x/oauth2 v0.0.0-20210112200429-01de73cf58bd
	gonum.org/v1/gonum v0.6.2 // indirect
	gonum.org/v1/netlib v0.0.0-20190331212654-76723241ea4e // indirect
	google.golang.org/api v0.36.0
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	gopkg.in/gcfg.v1 v1.2.0
	gopkg.in/warnings.v0 v0.1.2
	k8s.io/api v0.22.0
	k8s.io/apimachinery v0.22.0
	k8s.io/apiserver v0.22.0
	k8s.io/client-go v9.0.0+incompatible
	k8s.io/cloud-provider v0.22.0
	k8s.io/cloud-provider-gcp/providers v0.0.0
	k8s.io/component-base v0.22.0
	k8s.io/controller-manager v0.22.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kubectl v0.0.0 // indirect
	k8s.io/kubelet v0.22.0
	k8s.io/kubernetes v1.15.0-alpha.1
	k8s.io/legacy-cloud-providers v0.0.0-20210809163733-8cecffd58035 // indirect
	k8s.io/pod-security-admission v0.0.0 // indirect
	k8s.io/system-validators v1.5.0 // indirect
)

replace (
	cloud.google.com/go => cloud.google.com/go v0.75.0
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.19.6 // indirect
	github.com/go-openapi/swag => github.com/go-openapi/swag v0.19.7 // indirect
	github.com/gofrs/flock => github.com/gofrs/flock v0.7.1
	github.com/google/uuid => github.com/google/uuid v1.1.4 // indirect
	github.com/hashicorp/golang-lru => github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo => github.com/imdario/mergo v0.3.11 // indirect
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.3 => github.com/onsi/gomega v1.10.3 // indirect
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.7.1
	github.com/spf13/cobra => github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag => github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify => github.com/stretchr/testify v1.7.0
	go.uber.org/zap => go.uber.org/zap v1.14.1 // indirect
	golang.org/x/lint => golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20210112200429-01de73cf58bd
	golang.org/x/sync => golang.org/x/sync v0.0.0-20201207232520-09787c993a3a // indirect
	google.golang.org/api => google.golang.org/api v0.30.0
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20210111234610-22ae2b108f89 // indirect
	google.golang.org/grpc => google.golang.org/grpc v1.32.0 // indirect
	gopkg.in/check.v1 => gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	gopkg.in/gcfg.v1 => gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 => gopkg.in/warnings.v0 v0.1.2

	k8s.io/api => k8s.io/api v0.22.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.0
	k8s.io/apiserver => k8s.io/apiserver v0.22.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.0
	k8s.io/client-go => k8s.io/client-go v0.22.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.0
	k8s.io/cloud-provider-gcp/providers => ./providers
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.0
	k8s.io/code-generator => k8s.io/code-generator v0.22.0
	k8s.io/component-base => k8s.io/component-base v0.22.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.0
	k8s.io/cri-api => k8s.io/cri-api v0.22.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.0
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.8.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.0
	k8s.io/kubectl => k8s.io/kubectl v0.22.0
	k8s.io/kubelet => k8s.io/kubelet v0.22.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20210809163733-8cecffd58035
	k8s.io/metrics => k8s.io/metrics v0.22.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.0
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.0
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.22.0
	k8s.io/sample-controller => k8s.io/sample-controller v0.22.0
	k8s.io/utils => k8s.io/utils v0.0.0-20210802155522-efc7438f0176 // indirect
)
