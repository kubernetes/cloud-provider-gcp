# gazelle:prefix k8s.io/cloud-provider-gcp

load("@io_bazel_rules_go//go:def.bzl", "go_prefix")

go_prefix("k8s.io/cloud-provider-gcp")

load("@bazel_gazelle//:def.bzl", "gazelle")

gazelle(
    name = "gazelle",
    extra_args = [
        "-build_file_name=BUILD",
    ],
    prefix = "k8s.io/cloud-provider-gcp",
)
