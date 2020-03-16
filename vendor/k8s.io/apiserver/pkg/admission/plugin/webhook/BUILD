load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["accessors.go"],
    importmap = "k8s.io/kubernetes/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/webhook",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admissionregistration/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/labels:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/webhook:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
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
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/config:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/errors:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/generic:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/initializer:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/mutating:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/namespace:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/object:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/request:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/rules:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/testcerts:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/testing:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/util:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/validating:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["accessors_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/api/admissionregistration/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/diff:go_default_library",
        "//vendor/github.com/google/gofuzz:go_default_library",
    ],
)
