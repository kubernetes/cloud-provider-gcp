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
      "//tools:release-prod": "gcr.io",
      "//tools:release-devel": "{STABLE_DEVEL_REGISTRY}",
  })
  _image_repo = select({
      "//tools:release-prod": "k8s-image-staging",
      "//tools:release-devel": "{STABLE_DEVEL_REPO}",
  })
  container_push(
      name = "publish",
      format = "Docker",
      image = ":image",
      registry = _image_registry,
      repository = _image_repo + "/" + name,
      stamp = True,
      tag = "{STABLE_CERT_CONTROLLER_VERSION}",
  )

def _push_impl(ctx):
  output = ctx.outputs.out
  ctx.actions.run_shell(
      outputs = [ctx.outputs.out],
      use_default_shell_env = True,
      execution_requirements = {"local": "1", "no-cache": "1"},
      command="TMPDIR=/tmp gsutil cp -r %s %s/%s/ >%s" % (
              ctx.executable.src.path,
              ctx.attr.repo,
              ctx.attr.version,
              ctx.outputs.out.path,
      ),
  )

gcs_upload = rule(
    implementation=_push_impl,
    attrs={
        "src": attr.label(mandatory=True, executable=True, cfg="target"),
        "repo": attr.string(mandatory=True),
        "version": attr.string(mandatory=True),
    },
    outputs={"out": "%{name}.txt"},
)
