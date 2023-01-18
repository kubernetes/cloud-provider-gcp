.PHONY: build
build:
	kustomize build --load-restrictor LoadRestrictionsNone src/staticpod/ > manifest.yaml
	kustomize build --load-restrictor LoadRestrictionsNone src/default/ > packages/default/manifest.yaml
