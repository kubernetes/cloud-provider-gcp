package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
)
load("//defs:version.bzl", "version_x_defs")

go_binary(
    name = "gke-gcloud-auth-plugin",
    embed = [":gke-gcloud-auth-plugin_lib"],
    pure = "on",
    x_defs = version_x_defs(),
)

go_library(
    name = "gke-gcloud-auth-plugin_lib",
    srcs = [
        "cred.go",
        "default_credentials_token_provider.go",
        "gcloud_edge_cloud_token_provider.go",
        "gcloud_token_provider.go",
        "token_provider.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin",
    deps = [
        "//vendor/github.com/natefinch/atomic",
        "//vendor/github.com/spf13/pflag",
        "//vendor/golang.org/x/oauth2",
        "//vendor/golang.org/x/oauth2/google",
        "//vendor/google.golang.org/api/impersonate",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication/v1beta1",
        "//vendor/k8s.io/client-go/tools/clientcmd",
        "//vendor/k8s.io/client-go/tools/clientcmd/api",
        "//vendor/k8s.io/client-go/util/retry",
        "//vendor/k8s.io/component-base/version/verflag",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_test(
    name = "gke-gcloud-auth-plugin_test",
    srcs = ["cred_test.go"],
    embed = [":gke-gcloud-auth-plugin_lib"],
    deps = [
        "//vendor/github.com/google/go-cmp/cmp",
        "//vendor/golang.org/x/oauth2",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication/v1beta1",
        "//vendor/k8s.io/client-go/tools/clientcmd/api",
    ],
)
