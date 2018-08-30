load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "document.go",
        "openapi.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kube-openapi/pkg/util/proto",
    importpath = "k8s.io/kube-openapi/pkg/util/proto",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/googleapis/gnostic/OpenAPIv2:go_default_library",
        "//vendor/gopkg.in/yaml.v2:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "openapi_suite_test.go",
        "openapi_test.go",
    ],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/kube-openapi/pkg/util/proto/testing:go_default_library",
        "@com_github_onsi_ginkgo//:go_default_library",
        "@com_github_onsi_ginkgo//config:go_default_library",
        "@com_github_onsi_ginkgo//types:go_default_library",
        "@com_github_onsi_gomega//:go_default_library",
    ],
)
