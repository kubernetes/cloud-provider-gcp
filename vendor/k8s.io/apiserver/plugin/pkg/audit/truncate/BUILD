load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "truncate.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/plugin/pkg/audit/truncate",
    importpath = "k8s.io/apiserver/plugin/pkg/audit/truncate",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/errors:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/audit:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/audit:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["truncate_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/audit:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/v1beta1:go_default_library",
        "//vendor/k8s.io/apiserver/plugin/pkg/audit/fake:go_default_library",
        "//vendor/k8s.io/apiserver/plugin/pkg/audit/webhook:go_default_library",
        "@com_github_stretchr_testify//require:go_default_library",
    ],
)
