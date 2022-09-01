load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "generators",
    srcs = [
        "client_generator.go",
        "generator_for_clientset.go",
        "generator_for_expansion.go",
        "generator_for_group.go",
        "generator_for_type.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/client-gen/generators",
    importpath = "k8s.io/code-generator/cmd/client-gen/generators",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/code-generator/cmd/client-gen/args",
        "//vendor/k8s.io/code-generator/cmd/client-gen/generators/fake",
        "//vendor/k8s.io/code-generator/cmd/client-gen/generators/scheme",
        "//vendor/k8s.io/code-generator/cmd/client-gen/generators/util",
        "//vendor/k8s.io/code-generator/cmd/client-gen/path",
        "//vendor/k8s.io/code-generator/cmd/client-gen/types",
        "//vendor/k8s.io/code-generator/pkg/namer",
        "//vendor/k8s.io/code-generator/pkg/util",
        "//vendor/k8s.io/gengo/args",
        "//vendor/k8s.io/gengo/generator",
        "//vendor/k8s.io/gengo/namer",
        "//vendor/k8s.io/gengo/types",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
