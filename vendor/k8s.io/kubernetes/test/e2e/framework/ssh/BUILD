load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "ssh",
    srcs = ["ssh.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/test/e2e/framework/ssh",
    importpath = "k8s.io/kubernetes/test/e2e/framework/ssh",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/onsi/gomega",
        "//vendor/golang.org/x/crypto/ssh",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/fields",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/kubernetes/test/e2e/framework",
    ],
)
