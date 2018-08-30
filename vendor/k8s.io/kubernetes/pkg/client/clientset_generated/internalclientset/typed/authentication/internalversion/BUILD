package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "authentication_client.go",
        "doc.go",
        "generated_expansion.go",
        "tokenreview.go",
        "tokenreview_expansion.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authentication/internalversion",
    importpath = "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authentication/internalversion",
    deps = [
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/authentication:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/scheme:go_default_library",
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
        "//pkg/client/clientset_generated/internalclientset/typed/authentication/internalversion/fake:all-srcs",
    ],
    tags = ["automanaged"],
)
