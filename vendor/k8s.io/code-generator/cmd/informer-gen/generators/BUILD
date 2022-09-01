load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "generators",
    srcs = [
        "factory.go",
        "factoryinterface.go",
        "generic.go",
        "groupinterface.go",
        "informer.go",
        "packages.go",
        "types.go",
        "versioninterface.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/informer-gen/generators",
    importpath = "k8s.io/code-generator/cmd/informer-gen/generators",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/code-generator/cmd/client-gen/generators/util",
        "//vendor/k8s.io/code-generator/cmd/client-gen/types",
        "//vendor/k8s.io/code-generator/cmd/informer-gen/args",
        "//vendor/k8s.io/code-generator/pkg/namer",
        "//vendor/k8s.io/code-generator/pkg/util",
        "//vendor/k8s.io/gengo/args",
        "//vendor/k8s.io/gengo/generator",
        "//vendor/k8s.io/gengo/namer",
        "//vendor/k8s.io/gengo/types",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
