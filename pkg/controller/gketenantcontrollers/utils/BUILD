load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "utils",
    srcs = ["helpers.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/controller/gketenantcontrollers/utils",
    visibility = [
        "//cmd/cloud-controller-manager:__pkg__",
        "//pkg/controller/gketenantcontrollers:__subpackages__",
    ],
    deps = ["//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1:providerconfig"],
)

go_test(
    name = "utils_test",
    srcs = ["helpers_test.go"],
    embed = [":utils"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1:providerconfig",
        "//vendor/github.com/stretchr/testify/assert",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
    ],
)
