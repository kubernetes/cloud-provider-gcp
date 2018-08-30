package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)

go_binary(
    name = "go-to-protobuf",
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/go-to-protobuf",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/go-to-protobuf",
    deps = [
        "//vendor/github.com/spf13/pflag:go_default_library",
        "@io_k8s_code_generator//cmd/go-to-protobuf/protobuf:go_default_library",
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
        "//staging/src/k8s.io/code-generator/cmd/go-to-protobuf/protobuf:all-srcs",
        "//staging/src/k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo:all-srcs",
    ],
    tags = ["automanaged"],
)
