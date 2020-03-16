load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "events.go",
        "pod_backoff.go",
        "scheduling_queue.go",
    ],
    importpath = "k8s.io/kubernetes/pkg/scheduler/internal/queue",
    visibility = ["//vendor/k8s.io/kubernetes/pkg/scheduler:__subpackages__"],
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/api/v1/pod:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/algorithm/predicates:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/algorithm/priorities/util:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/internal/heap:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/metrics:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/util:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/client-go/tools/cache:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "pod_backoff_test.go",
        "scheduling_queue_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/api/v1/pod:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/apis/config:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/listers:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/metrics:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/nodeinfo:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/nodeinfo/snapshot:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/scheduler/util:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/clock:go_default_library",
        "//vendor/k8s.io/client-go/informers:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/component-base/metrics/testutil:go_default_library",
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
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
