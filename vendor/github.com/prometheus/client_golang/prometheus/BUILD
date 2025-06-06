load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "prometheus",
    srcs = [
        "build_info_collector.go",
        "collector.go",
        "collectorfunc.go",
        "counter.go",
        "desc.go",
        "doc.go",
        "expvar_collector.go",
        "fnv.go",
        "gauge.go",
        "get_pid.go",
        "go_collector.go",
        "go_collector_go116.go",
        "go_collector_latest.go",
        "histogram.go",
        "labels.go",
        "metric.go",
        "num_threads.go",
        "observer.go",
        "process_collector.go",
        "process_collector_darwin.go",
        "process_collector_mem_cgo_darwin.c",
        "process_collector_mem_cgo_darwin.go",
        "process_collector_mem_nocgo_darwin.go",
        "process_collector_not_supported.go",
        "process_collector_procfsenabled.go",
        "process_collector_windows.go",
        "registry.go",
        "summary.go",
        "timer.go",
        "untyped.go",
        "value.go",
        "vec.go",
        "vnext.go",
        "wrap.go",
    ],
    cgo = True,
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/prometheus/client_golang/prometheus",
    importpath = "github.com/prometheus/client_golang/prometheus",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/beorn7/perks/quantile",
        "//vendor/github.com/cespare/xxhash/v2:xxhash",
        "//vendor/github.com/prometheus/client_golang/prometheus/internal",
        "//vendor/github.com/prometheus/client_model/go",
        "//vendor/github.com/prometheus/common/expfmt",
        "//vendor/github.com/prometheus/common/model",
        "//vendor/google.golang.org/protobuf/proto",
        "//vendor/google.golang.org/protobuf/types/known/timestamppb",
    ] + select({
        "@io_bazel_rules_go//go/platform:aix": [
            "//vendor/github.com/prometheus/procfs",
        ],
        "@io_bazel_rules_go//go/platform:android": [
            "//vendor/github.com/prometheus/procfs",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:dragonfly": [
            "//vendor/github.com/prometheus/procfs",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "//vendor/github.com/prometheus/procfs",
        ],
        "@io_bazel_rules_go//go/platform:illumos": [
            "//vendor/github.com/prometheus/procfs",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/github.com/prometheus/procfs",
        ],
        "@io_bazel_rules_go//go/platform:netbsd": [
            "//vendor/github.com/prometheus/procfs",
        ],
        "@io_bazel_rules_go//go/platform:openbsd": [
            "//vendor/github.com/prometheus/procfs",
        ],
        "@io_bazel_rules_go//go/platform:plan9": [
            "//vendor/github.com/prometheus/procfs",
        ],
        "@io_bazel_rules_go//go/platform:solaris": [
            "//vendor/github.com/prometheus/procfs",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/golang.org/x/sys/windows",
        ],
        "//conditions:default": [],
    }),
)
