module k8s.io/cloud-provider-gcp

require (
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/apiserver v0.20.0
	k8s.io/client-go v0.20.0
	k8s.io/cloud-provider v0.20.0
	k8s.io/component-base v0.20.0
	k8s.io/kubelet v0.20.0
	k8s.io/kubernetes v1.20.0
	k8s.io/legacy-cloud-providers v0.20.0
)

replace (
	k8s.io/api => k8s.io/api v0.20.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.0
	k8s.io/apiserver => k8s.io/apiserver v0.20.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.20.0
	k8s.io/client-go => k8s.io/client-go v0.20.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.20.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.20.0
	k8s.io/code-generator => k8s.io/code-generator v0.20.0
	k8s.io/component-base => k8s.io/component-base v0.20.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.20.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.20.0
	k8s.io/cri-api => k8s.io/cri-api v0.20.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.20.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.20.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.20.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.20.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.20.0
	k8s.io/kubectl => k8s.io/kubectl v0.20.0
	k8s.io/kubelet => k8s.io/kubelet v0.20.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.20.0
	k8s.io/metrics => k8s.io/metrics v0.20.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.20.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.20.0
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.20.0
	k8s.io/sample-controller => k8s.io/sample-controller v0.20.0
)

require (
	cloud.google.com/go v0.75.0
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/gofrs/flock v0.7.1
	github.com/gogo/protobuf v1.3.1
	github.com/google/go-tpm v0.2.0
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.10.0
	github.com/shirou/gopsutil v3.20.10+incompatible // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200910180754-dd1b699fc489
	golang.org/x/oauth2 v0.0.0-20210112200429-01de73cf58bd
	google.golang.org/api v0.36.0
	google.golang.org/grpc v1.34.0
	google.golang.org/grpc/examples v0.0.0-20210122012134-2c42474aca0c // indirect
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2
	honnef.co/go/tools v0.0.1-2020.1.5 // indirect
	k8s.io/controller-manager v0.20.0
	k8s.io/klog/v2 v2.4.0
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009 // indirect
	sigs.k8s.io/kubetest2 v0.0.0-20210309183806-9230b4e73d8d // indirect
)

go 1.13
