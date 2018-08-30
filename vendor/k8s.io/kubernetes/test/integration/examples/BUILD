package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_test",
)

go_test(
    name = "go_default_test",
    size = "large",
    srcs = [
        "apiserver_test.go",
        "main_test.go",
        "setup_test.go",
        "webhook_test.go",
    ],
    tags = ["integration"],
    deps = [
        "//vendor/k8s.io/api/admissionregistration/v1beta1:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/audit:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server/options:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd/api:go_default_library",
        "//vendor/k8s.io/client-go/util/cert:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kube-apiserver/app:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kube-apiserver/app/options:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/master:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/master/reconcilers:go_default_library",
        "//vendor/k8s.io/kubernetes/test/integration/framework:go_default_library",
        "@com_github_pborman_uuid//:go_default_library",
        "@com_github_stretchr_testify//assert:go_default_library",
        "@io_k8s_kube_aggregator//pkg/apis/apiregistration/v1beta1:go_default_library",
        "@io_k8s_kube_aggregator//pkg/client/clientset_generated/clientset:go_default_library",
        "@io_k8s_kube_aggregator//pkg/cmd/server:go_default_library",
        "@io_k8s_sample_apiserver//pkg/apis/wardle/v1alpha1:go_default_library",
        "@io_k8s_sample_apiserver//pkg/apis/wardle/v1beta1:go_default_library",
        "@io_k8s_sample_apiserver//pkg/cmd/server:go_default_library",
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
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)
