load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_image",
    "container_push",
)

# image macro creates basic image and push rules for a main
def image(binary):
  if len(binary) == 0:
    fail("binary is a required argument")
  if binary[0] != ":":
    fail("binary must be a package local label")
  name = binary[1:]
  container_image(
      name = "image",
      cmd = ["/" + name],
      files = [":" + name],
  )
  _image_registry = select({
      "//tools:release-prod": "k8s.gcr.io",
      "//tools:release-devel": "{STABLE_DEVEL_REGISTRY}",
  })
  _image_repo = select({
      "//tools:release-prod": "google-containers",
      "//tools:release-devel": "{STABLE_DEVEL_REPO}",
  })
  container_push(
      name = "publish",
      format = "Docker",
      image = ":image",
      registry = _image_registry,
      repository = _image_repo + "/" + name,
      stamp = True,
      tag = "{STABLE_GIT_COMMIT}",
  )
