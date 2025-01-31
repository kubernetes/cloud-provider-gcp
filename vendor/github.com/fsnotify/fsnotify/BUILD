load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fsnotify",
    srcs = [
        "backend_fen.go",
        "backend_inotify.go",
        "backend_kqueue.go",
        "backend_other.go",
        "backend_windows.go",
        "fsnotify.go",
        "system_bsd.go",
        "system_darwin.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/fsnotify/fsnotify",
    importpath = "github.com/fsnotify/fsnotify",
    visibility = ["//visibility:public"],
    deps = select({
        "@io_bazel_rules_go//go/platform:android": [
            "//vendor/github.com/fsnotify/fsnotify/internal",
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "//vendor/github.com/fsnotify/fsnotify/internal",
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:dragonfly": [
            "//vendor/github.com/fsnotify/fsnotify/internal",
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "//vendor/github.com/fsnotify/fsnotify/internal",
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:ios": [
            "//vendor/github.com/fsnotify/fsnotify/internal",
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/github.com/fsnotify/fsnotify/internal",
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:netbsd": [
            "//vendor/github.com/fsnotify/fsnotify/internal",
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:openbsd": [
            "//vendor/github.com/fsnotify/fsnotify/internal",
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:solaris": [
            "//vendor/github.com/fsnotify/fsnotify/internal",
            "//vendor/golang.org/x/sys/unix",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/github.com/fsnotify/fsnotify/internal",
            "//vendor/golang.org/x/sys/windows",
        ],
        "//conditions:default": [],
    }),
)
