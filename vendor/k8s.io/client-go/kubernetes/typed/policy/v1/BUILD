load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "policy",
    srcs = [
        "doc.go",
        "eviction.go",
        "eviction_expansion.go",
        "generated_expansion.go",
        "poddisruptionbudget.go",
        "policy_client.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/policy/v1",
    importpath = "k8s.io/client-go/kubernetes/typed/policy/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/policy/v1:policy",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/applyconfigurations/policy/v1:policy",
        "//vendor/k8s.io/client-go/gentype",
        "//vendor/k8s.io/client-go/kubernetes/scheme",
        "//vendor/k8s.io/client-go/rest",
    ],
)
