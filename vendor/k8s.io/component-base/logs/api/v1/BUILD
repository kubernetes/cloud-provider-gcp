load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "api",
    srcs = [
        "doc.go",
        "kube_features.go",
        "options.go",
        "options_no_slog.go",
        "options_slog.go",
        "pflags.go",
        "registry.go",
        "text.go",
        "types.go",
        "zz_generated.deepcopy.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/logs/api/v1",
    importpath = "k8s.io/component-base/logs/api/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/go-logr/logr",
        "//vendor/github.com/google/go-cmp/cmp",
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/apimachinery/pkg/api/resource",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/component-base/cli/flag",
        "//vendor/k8s.io/component-base/featuregate",
        "//vendor/k8s.io/component-base/logs/internal/setverbositylevel",
        "//vendor/k8s.io/component-base/logs/klogflags",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/klog/v2/textlogger",
    ],
)
