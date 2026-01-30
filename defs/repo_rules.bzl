load("@bazel_skylib//lib:paths.bzl", "paths")

BUILD_PRELUDE = """
package(default_visibility = ["//visibility:public"])

load("@rules_pkg//pkg:tar.bzl", "pkg_tar")
"""

BUILD_TAR_TEMPLATE = """
pkg_tar(
    name = "{}",
    deps = [":{}"],
)
"""

def _archive_url(folder, version, archive):
    return paths.join("https://dl.k8s.io", folder, version, archive)

def _fetch_kube_release(ctx):
    build_file_contents = BUILD_PRELUDE
    for archive in ctx.attr.archives:
        ctx.download(
            url = _archive_url(ctx.attr.folder, ctx.attr.version, archive),
            output = archive,
            sha256 = ctx.attr.archives[archive],
        )
        build_file_contents += BUILD_TAR_TEMPLATE.format(
            paths.basename(archive).split(".")[0],
            archive,
        )
    ctx.file("BUILD", content = build_file_contents)

fetch_kube_release = repository_rule(
    implementation = _fetch_kube_release,
    attrs = {
        "folder": attr.string(default = "release"),
        "version": attr.string(mandatory = True),
        "archives": attr.string_dict(mandatory = True),
    },
)
