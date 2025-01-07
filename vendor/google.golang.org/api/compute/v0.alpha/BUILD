load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v0_alpha",
    srcs = [
        "compute-gen.go",
        "compute2-gen.go",
        "compute3-gen.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/api/compute/v0.alpha",
    importpath = "google.golang.org/api/compute/v0.alpha",
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
