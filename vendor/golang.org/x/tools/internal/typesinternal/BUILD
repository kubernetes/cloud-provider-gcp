load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "typesinternal",
    srcs = [
        "classify_call.go",
        "element.go",
        "errorcode.go",
        "errorcode_string.go",
        "fx.go",
        "isnamed.go",
        "qualifier.go",
        "recv.go",
        "toonew.go",
        "types.go",
        "varkind.go",
        "zerovalue.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/internal/typesinternal",
    importpath = "golang.org/x/tools/internal/typesinternal",
    visibility = ["//vendor/golang.org/x/tools:__subpackages__"],
    deps = [
        "//vendor/golang.org/x/tools/go/ast/inspector",
        "//vendor/golang.org/x/tools/go/types/typeutil",
        "//vendor/golang.org/x/tools/internal/aliases",
        "//vendor/golang.org/x/tools/internal/stdlib",
        "//vendor/golang.org/x/tools/internal/versions",
    ],
)
