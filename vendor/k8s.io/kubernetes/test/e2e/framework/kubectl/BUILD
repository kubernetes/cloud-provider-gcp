load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "kubectl",
    srcs = [
        "builder.go",
        "kubectl_utils.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/test/e2e/framework/kubectl",
    importpath = "k8s.io/kubernetes/test/e2e/framework/kubectl",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/onsi/ginkgo/v2:ginkgo",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/tools/clientcmd",
        "//vendor/k8s.io/kubernetes/test/e2e/framework",
        "//vendor/k8s.io/kubernetes/test/e2e/framework/pod",
        "//vendor/k8s.io/kubernetes/test/utils",
        "//vendor/k8s.io/utils/exec",
    ],
)
