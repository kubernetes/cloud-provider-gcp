load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["search.pb.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/appengine/internal/search",
    importpath = "google.golang.org/appengine/internal/search",
    visibility = ["//vendor/google.golang.org/appengine:__subpackages__"],
    deps = ["//vendor/github.com/golang/protobuf/proto:go_default_library"],
)
