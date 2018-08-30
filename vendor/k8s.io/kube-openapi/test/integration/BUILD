load("@io_bazel_rules_go//go:def.bzl", "go_test")

go_test(
    name = "go_default_test",
    srcs = ["integration_suite_test.go"],
    deps = [
        "@com_github_onsi_ginkgo//:go_default_library",
        "@com_github_onsi_gomega//:go_default_library",
        "@com_github_onsi_gomega//gexec:go_default_library",
    ],
)
