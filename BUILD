load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:proto disable_global
gazelle(
    name = "gazelle",
    extra_args = [
        "-build_file_name=BUILD",
    ],
    prefix = "k8s.io/cloud-provider-gcp",
)

gazelle(
    name = "gazelle-diff",
    extra_args = [
        "-build_file_name=BUILD",
        "-mode=diff",
    ],
    prefix = "k8s.io/cloud-provider-gcp",
)

# gazelle:exclude crd
# gazelle:exclude test/e2e
# gazelle:exclude test/integration
