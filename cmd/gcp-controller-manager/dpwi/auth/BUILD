load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "auth",
    srcs = [
        "auth.go",
        "auth_fake.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/auth",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/apiserver/pkg/server/options",
        "//vendor/k8s.io/apiserver/pkg/util/webhook",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/tools/clientcmd/api",
    ],
)

go_test(
    name = "auth_test",
    srcs = ["auth_test.go"],
    embed = [":auth"],
)
