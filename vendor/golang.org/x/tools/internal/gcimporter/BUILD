load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "gcimporter",
    srcs = [
        "bimport.go",
        "exportdata.go",
        "gcimporter.go",
        "iexport.go",
        "iimport.go",
        "iimport_go122.go",
        "predeclared.go",
        "support.go",
        "ureader_yes.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/internal/gcimporter",
    importpath = "golang.org/x/tools/internal/gcimporter",
    visibility = ["//vendor/golang.org/x/tools:__subpackages__"],
    deps = [
        "//vendor/golang.org/x/tools/go/types/objectpath",
        "//vendor/golang.org/x/tools/internal/aliases",
        "//vendor/golang.org/x/tools/internal/pkgbits",
        "//vendor/golang.org/x/tools/internal/typesinternal",
    ],
)
