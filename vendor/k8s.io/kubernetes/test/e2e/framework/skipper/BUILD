load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "skipper",
    srcs = ["skipper.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/test/e2e/framework/skipper",
    importpath = "k8s.io/kubernetes/test/e2e/framework/skipper",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/onsi/ginkgo/v2:ginkgo",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/util/version",
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/component-base/featuregate",
        "//vendor/k8s.io/kubernetes/test/e2e/framework",
        "//vendor/k8s.io/kubernetes/test/e2e/framework/node",
        "//vendor/k8s.io/kubernetes/test/e2e/framework/ssh",
    ],
)
