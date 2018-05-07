package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "backoff.go",
        "throttle.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/client-go/util/flowcontrol",
    importpath = "k8s.io/client-go/util/flowcontrol",
    deps = [
        "//vendor/github.com/juju/ratelimit:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/clock:go_default_library",
        "//vendor/k8s.io/client-go/util/integer:go_default_library",
    ],
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
