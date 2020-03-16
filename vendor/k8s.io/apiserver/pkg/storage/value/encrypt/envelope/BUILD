package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "envelope.go",
        "grpc_service.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope",
    importpath = "k8s.io/apiserver/pkg/storage/value/encrypt/envelope",
    deps = [
        "//vendor/k8s.io/apiserver/pkg/storage/value:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/v1beta1:go_default_library",
        "//vendor/github.com/hashicorp/golang-lru:go_default_library",
        "//vendor/golang.org/x/crypto/cryptobyte:go_default_library",
        "//vendor/google.golang.org/grpc:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "envelope_test.go",
        "grpc_service_unix_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/storage/value:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/aes:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:android": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:dragonfly": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:ios": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:nacl": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:netbsd": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:openbsd": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:plan9": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:solaris": [
            "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:go_default_library",
        ],
        "//conditions:default": [],
    }),
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
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/testing:all-srcs",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/v1beta1:all-srcs",
    ],
    tags = ["automanaged"],
)
