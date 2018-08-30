package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = ["kind_visitor.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubectl/apps",
    importpath = "k8s.io/kubernetes/pkg/kubectl/apps",
    deps = ["//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library"],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)

go_test(
    name = "go_default_xtest",
    srcs = [
        "apps_suite_test.go",
        "kind_visitor_test.go",
    ],
    deps = [
        ":go_default_library",
        "//vendor/github.com/onsi/ginkgo:go_default_library",
        "//vendor/github.com/onsi/ginkgo/config:go_default_library",
        "//vendor/github.com/onsi/ginkgo/types:go_default_library",
        "//vendor/github.com/onsi/gomega:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "apps_suite_test.go",
        "kind_visitor_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "@com_github_onsi_ginkgo//:go_default_library",
        "@com_github_onsi_ginkgo//config:go_default_library",
        "@com_github_onsi_ginkgo//types:go_default_library",
        "@com_github_onsi_gomega//:go_default_library",
    ],
)
