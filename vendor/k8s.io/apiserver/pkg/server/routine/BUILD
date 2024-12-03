load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "routine",
    srcs = ["routine.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/server/routine",
    importpath = "k8s.io/apiserver/pkg/server/routine",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apiserver/pkg/endpoints/request"],
)
