package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "metrics.go",
        "node_ipam_controller.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/controller/nodeipam",
    importpath = "k8s.io/kubernetes/pkg/controller/nodeipam",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime:go_default_library",
        "//vendor/k8s.io/client-go/informers/core/v1:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:go_default_library",
        "//vendor/k8s.io/client-go/listers/core/v1:go_default_library",
        "//vendor/k8s.io/client-go/tools/cache:go_default_library",
        "//vendor/k8s.io/client-go/tools/record:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/cloudprovider:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/nodeipam/ipam:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/nodeipam/ipam/sync:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/metrics:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//pkg/controller/nodeipam/ipam:all-srcs",
    ],
    tags = ["automanaged"],
)
