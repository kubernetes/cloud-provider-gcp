load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "buf.go",
        "cache.go",
        "const.go",
        "entry.go",
        "frame.go",
        "line.go",
        "open.go",
        "symbol.go",
        "type.go",
        "typeunit.go",
        "unit.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/dwarf",
    importpath = "cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/dwarf",
    visibility = ["//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent:__subpackages__"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "frame_test.go",
        "pclntab_test.go",
        "type_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/elf:go_default_library",
        "//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/macho:go_default_library",
    ],
)
