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
      stamp = True,
  )
  image_repo = "{STABLE_IMAGE_REPO}"
  container_push(
      name = "publish",
      format = "Docker",
      image = ":image",
      registry = "gcr.io",
      repository = image_repo + "/" + name,
      stamp = True,
      tag = "{STABLE_IMAGE_TAG}",
  )
