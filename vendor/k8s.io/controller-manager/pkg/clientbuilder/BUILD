load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "clientbuilder",
    srcs = [
        "client_builder.go",
        "client_builder_dynamic.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/controller-manager/pkg/clientbuilder",
    importpath = "k8s.io/controller-manager/pkg/clientbuilder",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/oauth2",
        "//vendor/k8s.io/api/authentication/v1:authentication",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/authentication/serviceaccount",
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:core",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/transport",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/clock",
        "//vendor/k8s.io/utils/pointer",
    ],
)
