package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["codec.go"],
    importmap = "k8s.io/kubernetes/vendor/k8s.io/apimachinery/pkg/api/apitesting",
    importpath = "k8s.io/apimachinery/pkg/api/apitesting",
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer/recognizer:go_default_library",
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
        "//vendor/k8s.io/apimachinery/pkg/api/apitesting/fuzzer:all-srcs",
        "//vendor/k8s.io/apimachinery/pkg/api/apitesting/naming:all-srcs",
        "//vendor/k8s.io/apimachinery/pkg/api/apitesting/roundtrip:all-srcs",
    ],
    tags = ["automanaged"],
)
