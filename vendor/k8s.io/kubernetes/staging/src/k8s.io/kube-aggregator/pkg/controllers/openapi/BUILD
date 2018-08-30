load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "aggregator.go",
        "controller.go",
        "downloader.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator/pkg/controllers/openapi",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator/pkg/controllers/openapi",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/user:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server:go_default_library",
        "//vendor/k8s.io/client-go/util/workqueue:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/aggregator:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/builder:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/common:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/handler:go_default_library",
        "@com_github_emicklei_go_restful//:go_default_library",
        "@com_github_go_openapi_spec//:go_default_library",
        "@io_k8s_kube_aggregator//pkg/apis/apiregistration:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["aggregator_test.go"],
    embed = [":go_default_library"],
    deps = [
        "@com_github_go_openapi_spec//:go_default_library",
        "@com_github_stretchr_testify//assert:go_default_library",
        "@io_k8s_kube_aggregator//pkg/apis/apiregistration:go_default_library",
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
