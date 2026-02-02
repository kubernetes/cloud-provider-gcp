load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "output",
    srcs = ["output.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/test/e2e/framework/pod/output",
    importpath = "k8s.io/kubernetes/test/e2e/framework/pod/output",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/onsi/ginkgo/v2:ginkgo",
        "//vendor/github.com/onsi/gomega",
        "//vendor/github.com/onsi/gomega/types",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/kubectl/pkg/util/podutils",
        "//vendor/k8s.io/kubernetes/test/e2e/framework",
        "//vendor/k8s.io/kubernetes/test/e2e/framework/kubectl",
        "//vendor/k8s.io/kubernetes/test/e2e/framework/pod",
    ],
)
