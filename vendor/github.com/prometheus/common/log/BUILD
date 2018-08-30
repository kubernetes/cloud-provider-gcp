load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "eventlog_formatter.go",
        "log.go",
        "syslog_formatter.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/prometheus/common/log",
    importpath = "github.com/prometheus/common/log",
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_sirupsen_logrus//:go_default_library",
        "@in_gopkg_alecthomas_kingpin_v2//:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/golang.org/x/sys/windows/svc/eventlog:go_default_library",
        ],
        "//conditions:default": [],
    }),
)

go_test(
    name = "go_default_test",
    srcs = [
        "log_test.go",
        "syslog_formatter_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "@com_github_sirupsen_logrus//:go_default_library",
    ],
)
