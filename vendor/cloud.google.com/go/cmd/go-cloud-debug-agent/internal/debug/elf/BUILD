load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "elf.go",
        "file.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/elf",
    importpath = "cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/elf",
    visibility = ["//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent:__subpackages__"],
    deps = ["//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/dwarf:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "elf_test.go",
        "file_test.go",
    ],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    deps = ["//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/dwarf:go_default_library"],
)
