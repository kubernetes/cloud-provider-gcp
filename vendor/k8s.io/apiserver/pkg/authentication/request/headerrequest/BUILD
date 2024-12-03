load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "headerrequest",
    srcs = [
        "requestheader.go",
        "requestheader_controller.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/request/headerrequest",
    importpath = "k8s.io/apiserver/pkg/authentication/request/headerrequest",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/equality",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/fields",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticator",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/x509",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/listers/core/v1:core",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/util/workqueue",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
