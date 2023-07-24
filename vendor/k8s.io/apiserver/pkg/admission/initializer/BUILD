load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "initializer",
    srcs = [
        "initializer.go",
        "interfaces.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/initializer",
    importpath = "k8s.io/apiserver/pkg/admission/initializer",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apiserver/pkg/admission",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
        "//vendor/k8s.io/apiserver/pkg/cel/openapi/resolver",
        "//vendor/k8s.io/apiserver/pkg/quota/v1:quota",
        "//vendor/k8s.io/client-go/dynamic",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/component-base/featuregate",
    ],
)
