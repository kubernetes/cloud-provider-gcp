load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = [
        "compile.go",
        "gocovmerge.go",
        "profiles_and_reports.go",
        "run.go",
        "test_suite.go",
        "utils.go",
        "verify_version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/onsi/ginkgo/v2/ginkgo/internal",
    importpath = "github.com/onsi/ginkgo/v2/ginkgo/internal",
    visibility = ["//vendor/github.com/onsi/ginkgo/v2/ginkgo:__subpackages__"],
    deps = [
        "//vendor/github.com/google/pprof/profile",
        "//vendor/github.com/onsi/ginkgo/v2/formatter",
        "//vendor/github.com/onsi/ginkgo/v2/ginkgo/command",
        "//vendor/github.com/onsi/ginkgo/v2/internal/parallel_support",
        "//vendor/github.com/onsi/ginkgo/v2/reporters",
        "//vendor/github.com/onsi/ginkgo/v2/types",
        "//vendor/golang.org/x/tools/cover",
    ],
)
