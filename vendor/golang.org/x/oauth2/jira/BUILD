load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["jira.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/oauth2/jira",
    importpath = "golang.org/x/oauth2/jira",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/oauth2:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["jira_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/jws:go_default_library",
    ],
)
