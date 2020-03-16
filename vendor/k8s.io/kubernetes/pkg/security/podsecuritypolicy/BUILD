package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "factory.go",
        "provider.go",
        "types.go",
    ],
    importpath = "k8s.io/kubernetes/pkg/security/podsecuritypolicy",
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/api/pod:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core/pods:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/features:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/apparmor:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/capabilities:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/group:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/seccomp:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/selinux:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/sysctl:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/user:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/util:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/securitycontext:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/api/policy/v1beta1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["provider_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core/v1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/features:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/apparmor:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/seccomp:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/util:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/api/policy/v1beta1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/diff:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
        "//vendor/k8s.io/component-base/featuregate/testing:go_default_library",
        "//vendor/github.com/stretchr/testify/assert:go_default_library",
        "//vendor/github.com/stretchr/testify/require:go_default_library",
        "//vendor/k8s.io/utils/pointer:go_default_library",
    ],
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
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/apparmor:all-srcs",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/capabilities:all-srcs",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/group:all-srcs",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/seccomp:all-srcs",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/selinux:all-srcs",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/sysctl:all-srcs",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/user:all-srcs",
        "//vendor/k8s.io/kubernetes/pkg/security/podsecuritypolicy/util:all-srcs",
    ],
    tags = ["automanaged"],
)
