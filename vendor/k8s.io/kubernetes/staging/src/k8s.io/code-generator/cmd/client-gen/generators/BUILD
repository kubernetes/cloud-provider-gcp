package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "client_generator.go",
        "generator_for_clientset.go",
        "generator_for_expansion.go",
        "generator_for_group.go",
        "generator_for_type.go",
        "tags.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/client-gen/generators",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/client-gen/generators",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "@io_k8s_code_generator//cmd/client-gen/args:go_default_library",
        "@io_k8s_code_generator//cmd/client-gen/generators/fake:go_default_library",
        "@io_k8s_code_generator//cmd/client-gen/generators/scheme:go_default_library",
        "@io_k8s_code_generator//cmd/client-gen/generators/util:go_default_library",
        "@io_k8s_code_generator//cmd/client-gen/path:go_default_library",
        "@io_k8s_code_generator//cmd/client-gen/types:go_default_library",
        "@io_k8s_gengo//args:go_default_library",
        "@io_k8s_gengo//generator:go_default_library",
        "@io_k8s_gengo//namer:go_default_library",
        "@io_k8s_gengo//types:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//staging/src/k8s.io/code-generator/cmd/client-gen/generators/fake:all-srcs",
        "//staging/src/k8s.io/code-generator/cmd/client-gen/generators/scheme:all-srcs",
        "//staging/src/k8s.io/code-generator/cmd/client-gen/generators/util:all-srcs",
    ],
    tags = ["automanaged"],
)
