load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "resource",
    srcs = [
        "auto.go",
        "builtin.go",
        "config.go",
        "container.go",
        "doc.go",
        "env.go",
        "host_id.go",
        "host_id_bsd.go",
        "host_id_darwin.go",
        "host_id_exec.go",
        "host_id_linux.go",
        "host_id_readfile.go",
        "host_id_unsupported.go",
        "host_id_windows.go",
        "os.go",
        "os_release_darwin.go",
        "os_release_unix.go",
        "os_unix.go",
        "os_unsupported.go",
        "os_windows.go",
        "process.go",
        "resource.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/resource",
    importpath = "go.opentelemetry.io/otel/sdk/resource",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/uuid",
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/sdk",
        "//vendor/go.opentelemetry.io/otel/sdk/internal/x",
        "//vendor/go.opentelemetry.io/otel/semconv/v1.26.0:v1_26_0",
    ] + select({
        "@io_bazel_rules_go//go/platform:aix": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:android": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:dragonfly": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:ios": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:netbsd": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:openbsd": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:solaris": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/golang.org/x/sys/windows/registry",
        ],
        "//conditions:default": [],
    }),
)
