load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "hack",
    srcs = ["tools.go"],
    importpath = "k8s.io/cloud-provider-gcp/crd/hack",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/code-generator/cmd/client-gen:client-gen_lib",
        "//vendor/k8s.io/code-generator/cmd/deepcopy-gen:deepcopy-gen_lib",
        "//vendor/k8s.io/code-generator/cmd/informer-gen:informer-gen_lib",
        "//vendor/k8s.io/code-generator/cmd/lister-gen:lister-gen_lib",
        "//vendor/sigs.k8s.io/controller-tools/cmd/controller-gen:controller-gen_lib",
    ],
)
