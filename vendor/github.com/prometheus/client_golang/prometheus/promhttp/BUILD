load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "promhttp",
    srcs = [
        "delegator.go",
        "http.go",
        "instrument_client.go",
        "instrument_server.go",
        "option.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/prometheus/client_golang/prometheus/promhttp",
    importpath = "github.com/prometheus/client_golang/prometheus/promhttp",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/prometheus/client_golang/internal/github.com/golang/gddo/httputil",
        "//vendor/github.com/prometheus/client_golang/prometheus",
        "//vendor/github.com/prometheus/client_golang/prometheus/promhttp/internal",
        "//vendor/github.com/prometheus/client_model/go",
        "//vendor/github.com/prometheus/common/expfmt",
    ],
)
