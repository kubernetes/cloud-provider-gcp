module k8s.io/cloud-provider-gcp

require (
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/apiserver v0.19.0
	k8s.io/client-go v0.19.0
	k8s.io/cloud-provider v0.19.0
	k8s.io/component-base v0.19.0
	k8s.io/kube-controller-manager v0.19.0
	k8s.io/kubernetes v1.19.0
	k8s.io/legacy-cloud-providers v0.19.0
)

replace (
	k8s.io/api => k8s.io/api v0.19.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.0
	k8s.io/apiserver => k8s.io/apiserver v0.19.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.0
	k8s.io/client-go => k8s.io/client-go v0.19.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.0
	k8s.io/code-generator => k8s.io/code-generator v0.19.0
	k8s.io/component-base => k8s.io/component-base v0.19.0
	k8s.io/cri-api => k8s.io/cri-api v0.19.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.0
	k8s.io/kubectl => k8s.io/kubectl v0.19.0
	k8s.io/kubelet => k8s.io/kubelet v0.19.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.0
	k8s.io/metrics => k8s.io/metrics v0.19.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.0
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.19.0
	k8s.io/sample-controller => k8s.io/sample-controller v0.19.0
)

require (
	cloud.google.com/go v0.51.0
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/Sirupsen/logrus v1.0.6 // indirect
	github.com/blang/semver v3.5.0+incompatible
	github.com/docker/docker v1.13.1 // indirect
	github.com/gofrs/flock v0.7.1
	github.com/gogo/protobuf v1.3.1
	github.com/google/go-tpm v0.2.0
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.10.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200819165624-17cef6e3e9d5
	golang.org/x/oauth2 v0.0.0-20191202225959-858c2ad4c8b6
	google.golang.org/api v0.15.1
	google.golang.org/grpc v1.27.0
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2
	k8s.io/apiextensions-apiserver v0.19.0 // indirect
	k8s.io/cluster-bootstrap v0.19.0 // indirect
	k8s.io/cri-api v0.19.0 // indirect
	k8s.io/klog v1.0.0
	k8s.io/kube-aggregator v0.19.0 // indirect
	k8s.io/kube-proxy v0.19.0 // indirect
	k8s.io/kube-scheduler v0.19.0 // indirect
	k8s.io/kubectl v0.19.0 // indirect
	k8s.io/kubelet v0.19.0 // indirect
	k8s.io/sample-apiserver v0.19.0 // indirect
	k8s.io/sample-cli-plugin v0.19.0 // indirect
	k8s.io/sample-controller v0.19.0 // indirect
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
)

go 1.13
