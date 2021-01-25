load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "controllermanager.go",
        "nodeipamcontroller.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager/app",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/cobra:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
        "//vendor/k8s.io/cloud-provider:go_default_library",
        "//vendor/k8s.io/cloud-provider/app:go_default_library",
        "//vendor/k8s.io/cloud-provider/app/config:go_default_library",
        "//vendor/k8s.io/cloud-provider/options:go_default_library",
        "//vendor/k8s.io/component-base/cli/flag:go_default_library",
         "//vendor/k8s.io/component-base/cli/globalflag:go_default_library",
        "//vendor/k8s.io/component-base/logs:go_default_library",
        "//vendor/k8s.io/component-base/metrics/prometheus/clientgo:go_default_library",
        "//vendor/k8s.io/component-base/metrics/prometheus/version:go_default_library",
        "//vendor/k8s.io/component-base/term:go_default_library",
        "//vendor/k8s.io/component-base/version/verflag:go_default_library",
        "//vendor/k8s.io/controller-manager/app:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kube-controller-manager/app/options:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/nodeipam:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/nodeipam/config:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/nodeipam/ipam:go_default_library",
        "//vendor/k8s.io/legacy-cloud-providers/gce:go_default_library",
        "//vendor/k8s.io/utils/net:go_default_library",
    ],
)

