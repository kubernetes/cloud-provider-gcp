load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "deployment",
    srcs = [
        "fixtures.go",
        "wait.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/test/e2e/framework/deployment",
    importpath = "k8s.io/kubernetes/test/e2e/framework/deployment",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/apps/v1:apps",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/equality",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/util/uuid",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/kubernetes/test/e2e/framework",
        "//vendor/k8s.io/kubernetes/test/e2e/framework/pod",
        "//vendor/k8s.io/kubernetes/test/utils",
        "//vendor/k8s.io/pod-security-admission/api",
    ],
)
