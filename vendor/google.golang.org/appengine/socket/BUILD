load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "socket_vm.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/appengine/socket",
    importpath = "google.golang.org/appengine/socket",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/net/context:go_default_library"],
)
