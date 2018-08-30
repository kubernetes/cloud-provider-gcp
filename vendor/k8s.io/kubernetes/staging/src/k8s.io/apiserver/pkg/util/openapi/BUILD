load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["proto.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/util/openapi",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/util/openapi",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/googleapis/gnostic/OpenAPIv2:go_default_library",
        "//vendor/github.com/googleapis/gnostic/compiler:go_default_library",
        "//vendor/gopkg.in/yaml.v2:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/util/proto:go_default_library",
        "@com_github_go_openapi_spec//:go_default_library",
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
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["proto_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/util/proto:go_default_library",
        "@com_github_go_openapi_spec//:go_default_library",
    ],
)
