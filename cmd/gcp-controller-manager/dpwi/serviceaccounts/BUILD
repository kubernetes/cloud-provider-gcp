load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "serviceaccounts",
    srcs = [
        "handler.go",
        "types.go",
        "verifier.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/serviceaccounts",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/gcp-controller-manager/dpwi/ctxlog",
        "//cmd/gcp-controller-manager/dpwi/eventhandler",
        "//cmd/gcp-controller-manager/dpwi/hms",
        "//vendor/golang.org/x/sync/singleflight",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)

go_test(
    name = "serviceaccounts_test",
    srcs = [
        "handler_test.go",
        "verifier_test.go",
    ],
    embed = [":serviceaccounts"],
    deps = [
        "//cmd/gcp-controller-manager/dpwi/hms",
        "//vendor/github.com/google/go-cmp/cmp",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
