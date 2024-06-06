load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v0_beta",
    srcs = ["compute-gen.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/api/compute/v0.beta",
    importpath = "google.golang.org/api/compute/v0.beta",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/api/googleapi",
        "//vendor/google.golang.org/api/internal",
        "//vendor/google.golang.org/api/internal/gensupport",
        "//vendor/google.golang.org/api/option",
        "//vendor/google.golang.org/api/option/internaloption",
        "//vendor/google.golang.org/api/transport/http",
    ],
)
