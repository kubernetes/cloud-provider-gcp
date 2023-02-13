load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_image",
    "container_push",
)
load(
    "@io_bazel_rules_docker//container:bundle.bzl",
    "container_bundle",
)

# image macro creates basic image and push rules for a main
def image(binary, visibility = ["//visibility:public"]):
    if len(binary) == 0:
        fail("binary is a required argument")
    if binary[0] != ":":
        fail("binary must be a package local label")
    name = binary[1:]
    container_image(
        name = "image",
        repository = "registry.k8s.io",
        cmd = ["/" + name],
        files = [binary],
        stamp = "@io_bazel_rules_docker//stamp:always",
        base = "@go-runner//image",
        visibility = visibility,
    )
    image_registry = "{STABLE_IMAGE_REGISTRY}"
    image_repo = "{STABLE_IMAGE_REPO}"
    repository = image_repo + "/" + name
    container_push(
        name = "publish",
        format = "Docker",
        image = ":image",
        registry = image_registry,
        repository = repository,
        stamp = "@io_bazel_rules_docker//stamp:always",
        tag = "{STABLE_IMAGE_TAG}",
    )
    container_bundle(
        name = "bundle",
        images = {
            image_registry + "/" + repository + ":{STABLE_IMAGE_TAG}": ":image",
        },
        stamp = "@io_bazel_rules_docker//stamp:always",
        visibility = visibility,
    )
    native.genrule(
        name = "docker-tag",
        srcs = [":image"],
        outs = [name + ".docker_tag"],
        cmd = "awk '/STABLE_IMAGE_TAG/ {print $$2}' bazel-out/stable-status.txt >$@",
        stamp = 1,
        visibility = visibility,
    )
