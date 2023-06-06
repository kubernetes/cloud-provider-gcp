load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "nodesyncer",
    srcs = ["nodesyncer.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/nodesyncer",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/gcp-controller-manager/dpwi/ctxlog",
        "//cmd/gcp-controller-manager/dpwi/eventhandler",
        "//cmd/gcp-controller-manager/dpwi/hms",
        "//cmd/gcp-controller-manager/dpwi/serviceaccounts",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/tools/clientcmd/api",
        "//vendor/k8s.io/client-go/util/workqueue",
    ],
)

go_test(
    name = "nodesyncer_test",
    srcs = ["nodesyncer_test.go"],
    embed = [":nodesyncer"],
    deps = [
        "//cmd/gcp-controller-manager/dpwi/hms",
        "//cmd/gcp-controller-manager/dpwi/serviceaccounts",
        "//vendor/github.com/google/go-cmp/cmp",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
