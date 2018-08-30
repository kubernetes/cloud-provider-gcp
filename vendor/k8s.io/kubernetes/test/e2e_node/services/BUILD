package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "apiserver.go",
        "etcd.go",
        "internal_services.go",
        "kubelet.go",
        "logs.go",
        "namespace_controller.go",
        "server.go",
        "services.go",
        "util.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/test/e2e_node/services",
    importpath = "k8s.io/kubernetes/test/e2e_node/services",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flag:go_default_library",
        "//vendor/k8s.io/client-go/dynamic:go_default_library",
        "//vendor/k8s.io/client-go/informers:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kube-apiserver/app:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kube-apiserver/app/options:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubelet/app/options:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/namespace:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/features:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig/v1beta1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/kubeletconfig/util/codec:go_default_library",
        "//vendor/k8s.io/kubernetes/test/e2e/framework:go_default_library",
        "//vendor/k8s.io/kubernetes/test/e2e_node/builder:go_default_library",
        "//vendor/k8s.io/kubernetes/test/e2e_node/remote:go_default_library",
        "@com_github_coreos_etcd//etcdserver:go_default_library",
        "@com_github_coreos_etcd//etcdserver/api/v2http:go_default_library",
        "@com_github_coreos_etcd//pkg/transport:go_default_library",
        "@com_github_coreos_etcd//pkg/types:go_default_library",
        "@com_github_kardianos_osext//:go_default_library",
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
)
