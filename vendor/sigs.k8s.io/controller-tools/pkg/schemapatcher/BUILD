load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "schemapatcher",
    srcs = [
        "gen.go",
        "zz_generated.markerhelp.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/controller-tools/pkg/schemapatcher",
    importpath = "sigs.k8s.io/controller-tools/pkg/schemapatcher",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/gopkg.in/yaml.v3:yaml_v3",
        "//vendor/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1:apiextensions",
        "//vendor/k8s.io/apimachinery/pkg/api/equality",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/sigs.k8s.io/controller-tools/pkg/crd",
        "//vendor/sigs.k8s.io/controller-tools/pkg/crd/markers",
        "//vendor/sigs.k8s.io/controller-tools/pkg/genall",
        "//vendor/sigs.k8s.io/controller-tools/pkg/loader",
        "//vendor/sigs.k8s.io/controller-tools/pkg/markers",
        "//vendor/sigs.k8s.io/controller-tools/pkg/schemapatcher/internal/yaml",
        "//vendor/sigs.k8s.io/yaml",
    ],
)
