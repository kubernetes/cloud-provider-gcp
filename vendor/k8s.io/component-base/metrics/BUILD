load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "collector.go",
        "counter.go",
        "desc.go",
        "gauge.go",
        "histogram.go",
        "http.go",
        "labels.go",
        "metric.go",
        "opts.go",
        "processstarttime.go",
        "registry.go",
        "summary.go",
        "value.go",
        "version.go",
        "version_parser.go",
        "wrappers.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/k8s.io/component-base/metrics",
    importpath = "k8s.io/component-base/metrics",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/version:go_default_library",
        "//vendor/k8s.io/component-base/version:go_default_library",
        "//vendor/github.com/blang/semver:go_default_library",
        "//vendor/github.com/prometheus/client_golang/prometheus:go_default_library",
        "//vendor/github.com/prometheus/client_golang/prometheus/promhttp:go_default_library",
        "//vendor/github.com/prometheus/client_model/go:go_default_library",
        "//vendor/github.com/prometheus/procfs:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "collector_test.go",
        "counter_test.go",
        "gauge_test.go",
        "histogram_test.go",
        "opts_test.go",
        "registry_test.go",
        "summary_test.go",
        "version_parser_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/version:go_default_library",
        "//vendor/github.com/blang/semver:go_default_library",
        "//vendor/github.com/prometheus/client_golang/prometheus:go_default_library",
        "//vendor/github.com/prometheus/client_golang/prometheus/testutil:go_default_library",
        "//vendor/github.com/prometheus/client_model/go:go_default_library",
        "//vendor/github.com/prometheus/common/expfmt:go_default_library",
        "//vendor/github.com/stretchr/testify/assert:go_default_library",
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
        "//vendor/k8s.io/component-base/metrics/legacyregistry:all-srcs",
        "//vendor/k8s.io/component-base/metrics/prometheus/clientgo:all-srcs",
        "//vendor/k8s.io/component-base/metrics/prometheus/ratelimiter:all-srcs",
        "//vendor/k8s.io/component-base/metrics/prometheus/restclient:all-srcs",
        "//vendor/k8s.io/component-base/metrics/prometheus/version:all-srcs",
        "//vendor/k8s.io/component-base/metrics/prometheus/workqueue:all-srcs",
        "//vendor/k8s.io/component-base/metrics/testutil:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)

# Packages allowed to import github.com/prometheus
package_group(
    name = "prometheus_import_allow_list",
    packages = [
        "//cluster/images/etcd-version-monitor",
        "//vendor/k8s.io/kubernetes/pkg/controller/endpointslice",
        "//vendor/k8s.io/kubernetes/pkg/controller/volume/attachdetach/metrics",
        "//vendor/k8s.io/kubernetes/pkg/controller/volume/persistentvolume/metrics",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/metrics/collectors",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/pleg",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/pluginmanager/metrics",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/server",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/server/stats",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/volumemanager/metrics",
        "//vendor/k8s.io/kubernetes/pkg/master",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1",
        "//vendor/k8s.io/kubernetes/pkg/volume/util/operationexecutor",
        "//vendor/k8s.io/apiserver/pkg/admission/metrics",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/metrics",
        "//vendor/k8s.io/component-base/metrics/...",
        "//test/e2e",
        "//test/e2e/storage",
        "//test/e2e_node",
        "//vendor/...",
    ],
)
