load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["validation.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation",
    importpath = "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/validation:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field:go_default_library",
        "//vendor/k8s.io/client-go/tools/bootstrap/token/api:go_default_library",
        "//vendor/k8s.io/client-go/tools/bootstrap/token/util:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/constants:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/features:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/util:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core/validation:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig/scheme:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig/validation:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig/scheme:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig/validation:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/registry/core/service/ipallocator:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["validation_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig/v1beta1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/pointer:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
