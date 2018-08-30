package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_test(
    name = "go_default_test",
    size = "large",
    srcs = [
        "ipam_test.go",
        "main_test.go",
    ],
    embed = [":go_default_library"],
    tags = ["integration"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/client-go/informers:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/nodeipam:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/nodeipam/ipam:go_default_library",
        "//vendor/k8s.io/kubernetes/test/integration/framework:go_default_library",
        "//vendor/k8s.io/kubernetes/test/integration/util:go_default_library",
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

go_library(
    name = "go_default_library",
    srcs = [
        "cloud.go",
        "results.go",
        "util.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/test/integration/ipamperf",
    importpath = "k8s.io/kubernetes/test/integration/ipamperf",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/google.golang.org/api/compute/v0.beta:go_default_library",
        "//vendor/google.golang.org/api/compute/v1:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/client-go/informers:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/client-go/tools/cache:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/cloudprovider:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/cloudprovider/providers/gce/cloud:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/cloudprovider/providers/gce/cloud/meta:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/nodeipam/ipam:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/nodeipam/ipam/cidrset:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/util/node:go_default_library",
        "//vendor/k8s.io/kubernetes/test/integration/util:go_default_library",
    ],
)
