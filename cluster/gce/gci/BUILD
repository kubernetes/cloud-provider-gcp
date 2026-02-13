load("@io_bazel_rules_go//go:def.bzl", "go_test")
load("@rules_pkg//pkg:tar.bzl", "pkg_tar")
load("@io_k8s_repo_infra//defs:build.bzl", "release_filegroup")

# Having the COS code from the GCE cluster deploy hosted with the release is
# useful for GKE.  This list should match the list in
# kubernetes/release/lib/releaselib.sh.
release_filegroup(
    name = "gcs-release-artifacts",
    srcs = [
        "configure.sh",
        "master.yaml",
        "node.yaml",
        "shutdown.sh",
    ],
    visibility = ["//visibility:public"],
)

pkg_tar(
    name = "gci-trusty-manifests",
    srcs = glob(["gke-internal-configure-helper.sh"]),
    files = {
        "//cluster/gce/gci/mounter": "gci-mounter",
        "configure-helper.sh": "gci-configure-helper.sh",
        "configure-kubeapiserver.sh": "configure-kubeapiserver.sh",
    },
    mode = "0755",
    strip_prefix = ".",
    visibility = ["//visibility:public"],
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
        "//cluster/gce/gci/mounter:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)

filegroup(
    name = "scripts-test-data",
    srcs = [
        "configure-helper.sh",
        "configure-kubeapiserver.sh",
    ],
)

filegroup(
    name = "testdata",
    srcs = glob(["testdata/**"]),
)

go_test(
    name = "go_default_test",
    srcs = [
        "apiserver_etcd_test.go",
        "apiserver_kms_test.go",
        "append_or_replace_prefixed_line_test.go",
        "audit_policy_test.go",
        "configure_helper_test.go",
    ],
    data = [
        ":scripts-test-data",
        ":testdata",
        "//cluster/gce/manifests",
    ],
    deps = [
        "//vendor/github.com/google/go-cmp/cmp",
        "//vendor/github.com/stretchr/testify/assert",
        "//vendor/github.com/stretchr/testify/require",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apiserver/pkg/apis/audit",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/install",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/apiserver/pkg/audit/policy",
        "//vendor/k8s.io/apiserver/pkg/authentication/serviceaccount",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
        "//vendor/k8s.io/kubernetes/pkg/api/legacyscheme",
    ],
)

go_test(
    name = "gci_test",
    srcs = [
        "apiserver_etcd_test.go",
        "apiserver_kms_test.go",
        "append_or_replace_prefixed_line_test.go",
        "audit_policy_test.go",
        "configure_helper_test.go",
    ],
    data = ["//cluster/gce/manifests"] + glob(["testdata/**"]) + glob(["*.sh"]),
    deps = [
        "//vendor/github.com/google/go-cmp/cmp",
        "//vendor/github.com/stretchr/testify/assert",
        "//vendor/github.com/stretchr/testify/require",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apiserver/pkg/apis/audit",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/install",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/apiserver/pkg/audit/policy",
        "//vendor/k8s.io/apiserver/pkg/authentication/serviceaccount",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
        "//vendor/k8s.io/kubernetes/pkg/api/legacyscheme",
    ],
)
