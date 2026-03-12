load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "gketenantcontrollers",
    srcs = [
        "cloud.go",
        "starter.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/pkg/controller/gketenantcontrollers",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/controller/gketenantcontrollers/utils",
        "//providers/gce",
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1:providerconfig",
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/filtered",
        "//vendor/github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud",
        "//vendor/gopkg.in/gcfg.v1:gcfg_v1",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/dynamic",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:core",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/tools/record",
        "//vendor/k8s.io/cloud-provider",
        "//vendor/k8s.io/cloud-provider/app/config",
        "//vendor/k8s.io/controller-manager/app",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/clock",
    ],
)

go_test(
    name = "gketenantcontrollers_test",
    srcs = [
        "cloud_test.go",
        "starter_test.go",
    ],
    embed = [":gketenantcontrollers"],
    deps = [
        "//providers/gce",
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1:providerconfig",
        "//vendor/github.com/stretchr/testify/assert",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/client-go/dynamic/fake",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/kubernetes/fake",
        "//vendor/k8s.io/client-go/tools/record",
        "//vendor/k8s.io/cloud-provider/app/config",
        "//vendor/k8s.io/controller-manager/app",
    ],
)
