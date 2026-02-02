load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "junit",
    srcs = ["junit.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/test/e2e/framework/internal/junit",
    importpath = "k8s.io/kubernetes/test/e2e/framework/internal/junit",
    visibility = ["//vendor/k8s.io/kubernetes/test/e2e/framework:__subpackages__"],
    deps = [
        "//vendor/github.com/onsi/ginkgo/v2:ginkgo",
        "//vendor/github.com/onsi/ginkgo/v2/reporters",
        "//vendor/github.com/onsi/ginkgo/v2/types",
    ],
)
