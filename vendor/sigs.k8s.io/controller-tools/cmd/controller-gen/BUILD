load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "controller-gen_lib",
    srcs = ["main.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/controller-tools/cmd/controller-gen",
    importpath = "sigs.k8s.io/controller-tools/cmd/controller-gen",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/spf13/cobra",
        "//vendor/sigs.k8s.io/controller-tools/pkg/crd",
        "//vendor/sigs.k8s.io/controller-tools/pkg/deepcopy",
        "//vendor/sigs.k8s.io/controller-tools/pkg/genall",
        "//vendor/sigs.k8s.io/controller-tools/pkg/genall/help",
        "//vendor/sigs.k8s.io/controller-tools/pkg/genall/help/pretty",
        "//vendor/sigs.k8s.io/controller-tools/pkg/markers",
        "//vendor/sigs.k8s.io/controller-tools/pkg/rbac",
        "//vendor/sigs.k8s.io/controller-tools/pkg/schemapatcher",
        "//vendor/sigs.k8s.io/controller-tools/pkg/version",
        "//vendor/sigs.k8s.io/controller-tools/pkg/webhook",
    ],
)

go_binary(
    name = "controller-gen",
    embed = [":controller-gen_lib"],
    visibility = ["//visibility:public"],
)
