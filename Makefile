.PHONY: release-tars

release-tars:
	# 1. Build the artifacts using Bazel
	bazel build //release:release-tars

	# 2. Prepare the output directory expected by kubetest2
	mkdir -p _output/release-tars

	# 3. Copy Linux artifacts (These should always exist on Linux builds)
	cp bazel-bin/release/kubernetes-server-linux-amd64.tar.gz _output/release-tars/
	cp bazel-bin/release/kubernetes-server-linux-amd64.tar.gz.sha512 _output/release-tars/
	cp bazel-bin/release/kubernetes-manifests.tar.gz _output/release-tars/
	cp bazel-bin/release/kubernetes-manifests.tar.gz.sha512 _output/release-tars/

	# 4. Copy Windows artifacts ONLY if they were built
	if [ -f bazel-bin/release/kubernetes-node-windows-amd64.tar.gz ]; then \
		cp bazel-bin/release/kubernetes-node-windows-amd64.tar.gz _output/release-tars/; \
		cp bazel-bin/release/kubernetes-node-windows-amd64.tar.gz.sha512 _output/release-tars/; \
	fi