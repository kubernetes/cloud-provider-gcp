load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "gketenantcontrollers",
    srcs = [
        "cloud.go",
        "constants.go",
        "filteredcache.go",
        "filteredfactory.go",
        "filteredinformer.go",
        "helpers.go",
        "starter.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/pkg/controller/gketenantcontrollers",
    visibility = ["//visibility:public"],
    deps = [
        "//providers/gce",
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1:providerconfig",
        "//vendor/gopkg.in/gcfg.v1:gcfg_v1",
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/informers/coordination",
        "//vendor/k8s.io/client-go/informers/coordination/v1:coordination",
        "//vendor/k8s.io/client-go/informers/core",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/listers/coordination/v1:coordination",
        "//vendor/k8s.io/client-go/listers/core/v1:core",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/cloud-provider",
        "//vendor/k8s.io/cloud-provider/app/config",
        "//vendor/k8s.io/controller-manager/app",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_test(
    name = "gketenantcontrollers_test",
    srcs = [
        "cloud_test.go",
        "filteredinformer_test.go",
        "helpers_test.go",
        "starter_test.go",
    ],
    embed = [":gketenantcontrollers"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1:providerconfig",
        "//vendor/github.com/stretchr/testify/assert",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
