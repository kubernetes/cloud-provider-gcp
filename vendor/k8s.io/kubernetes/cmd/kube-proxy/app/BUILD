package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "conntrack.go",
        "init_others.go",
        "init_windows.go",
        "server.go",
        "server_others.go",
        "server_windows.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cmd/kube-proxy/app",
    importpath = "k8s.io/kubernetes/cmd/kube-proxy/app",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/github.com/prometheus/client_golang/prometheus:go_default_library",
        "//vendor/github.com/spf13/cobra:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer/json:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server/healthz:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server/mux:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server/routes:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flag:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd/api:go_default_library",
        "//vendor/k8s.io/client-go/tools/record:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/componentconfig:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/qos:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/master/ports:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig/scheme:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig/validation:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/config:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/healthcheck:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/iptables:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/userspace:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/configz:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/flag:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/ipset:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/ipvs:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/mount:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/node:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/oom:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/pointer:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/resourcecontainer:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/sysctl:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/version:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/version/verflag:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:android": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:dragonfly": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:nacl": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:netbsd": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:openbsd": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:plan9": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:solaris": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/winkernel:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/winuserspace:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/netsh:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/windows/service:go_default_library",
        ],
        "//conditions:default": [],
    }),
)

go_test(
    name = "go_default_test",
    srcs = [
        "server_others_test.go",
        "server_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/diff:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/configz:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/pointer:go_default_library",
        "@com_github_stretchr_testify//assert:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:android": [
            "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:dragonfly": [
            "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:nacl": [
            "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:netbsd": [
            "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:openbsd": [
            "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:plan9": [
            "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:solaris": [
            "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        ],
        "//conditions:default": [],
    }),
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
