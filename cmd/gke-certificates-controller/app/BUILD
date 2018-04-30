load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "gke_approver.go",
        "gke_certificates_controller.go",
        "gke_signer.go",
        "options.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gke-certificates-controller/app",
    visibility = [
        "//cmd/gke-certificates-controller:__subpackages__",
    ],
    deps = [
        "//vendor/cloud.google.com/go/compute/metadata:go_default_library",
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/github.com/spf13/cobra:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/google.golang.org/api/compute/v1:go_default_library",
        "//vendor/gopkg.in/gcfg.v1:go_default_library",
        "//vendor/gopkg.in/warnings.v0:go_default_library",
        "//vendor/k8s.io/api/authorization/v1beta1:go_default_library",
        "//vendor/k8s.io/api/certificates/v1beta1:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/webhook:go_default_library",
        "//vendor/k8s.io/client-go/informers:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:go_default_library",
        "//vendor/k8s.io/client-go/plugin/pkg/client/auth/gcp:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd:go_default_library",
        "//vendor/k8s.io/client-go/tools/record:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/api/legacyscheme:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/certificates/install:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/certificates/v1beta1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/cloudprovider/providers/gce:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/certificates:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "gke_approver_test.go",
        "gke_signer_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/google.golang.org/api/compute/v1:go_default_library",
        "//vendor/k8s.io/api/authorization/v1beta1:go_default_library",
        "//vendor/k8s.io/api/certificates/v1beta1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes/fake:go_default_library",
        "//vendor/k8s.io/client-go/testing:go_default_library",
        "//vendor/k8s.io/client-go/tools/record:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/certificates/v1beta1:go_default_library",
    ],
)
