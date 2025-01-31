load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "validations",
    srcs = [
        "counter_validations.go",
        "duplicate_validations.go",
        "generic_name_validations.go",
        "help_validations.go",
        "histogram_validations.go",
        "units.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/prometheus/client_golang/prometheus/testutil/promlint/validations",
    importpath = "github.com/prometheus/client_golang/prometheus/testutil/promlint/validations",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/prometheus/client_model/go"],
)
