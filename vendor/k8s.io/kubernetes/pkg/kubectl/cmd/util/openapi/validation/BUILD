package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = ["validation.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi/validation",
    importpath = "k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi/validation",
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/json:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/yaml:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/util/proto/validation:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "validation_suite_test.go",
        "validation_test.go",
    ],
    data = ["//api/openapi-spec:swagger-spec"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/errors:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/util/proto/testing:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/util/proto/validation:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/api/testapi:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi:go_default_library",
        "@com_github_onsi_ginkgo//:go_default_library",
        "@com_github_onsi_ginkgo//config:go_default_library",
        "@com_github_onsi_ginkgo//types:go_default_library",
        "@com_github_onsi_gomega//:go_default_library",
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
)
