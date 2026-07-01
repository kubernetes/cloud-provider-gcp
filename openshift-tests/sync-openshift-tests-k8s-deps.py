#!/usr/bin/env python3

"""
Sync the Kubernetes staging `replace` directives in `openshift-tests/go.mod`
to match the parent module's Kubernetes version policy.

For each `k8s.io/*` replace in `openshift-tests`:
  - if the parent `go.mod` replaces that module, use the parent's exact
    replacement target
  - otherwise, if the parent `go.mod` requires that module, use the parent's
    version for that module
  - otherwise, use the parent module's `k8s.io/cloud-provider` version

This keeps `openshift-tests` following the parent module's chosen Kubernetes
version without putting `openshift-tests` into `go.work`.

Invoke from the repository root with either:
  ./openshift-tests/sync-openshift-tests-k8s-deps.py
or:
  make -C openshift-tests vendor

The make target is executed automatically by rebasebot on every rebase. It also
runs `go mod tidy` and `go mod vendor` to ensure the vendor directory is up to
date.
"""

import json
import shlex
import subprocess
import sys
from pathlib import Path

# This repo's own module namespace must never be treated as an upstream k8s.io
# staging module for sync purposes. We exclude this exact module path and its
# submodules, such as `k8s.io/cloud-provider-gcp/providers`, from the k8s pin
# logic.
LOCAL_REPO_MODULE = "k8s.io/cloud-provider-gcp"

# `k8s.io/cloud-provider` is the fallback anchor for Kubernetes staging module
# versions. When `openshift-tests` has a `k8s.io/*` replace that is not
# explicitly versioned by the parent module, we pin it to this module's version.
K8S_CLOUD_PROVIDER_MODULE = "k8s.io/cloud-provider"


def is_module_or_submodule(path, module_path):
    return path == module_path or path.startswith(f"{module_path}/")


def is_root_k8s_module_path(path):
    # Only sync the root-owned k8s staging pins. Preserve local repo replaces
    # such as ../providers and any OpenShift-specific overrides.
    return path.startswith("k8s.io/") and not is_module_or_submodule(path, LOCAL_REPO_MODULE)


def is_root_k8s_replace(replace):
    return is_root_k8s_module_path(replace["Old"]["Path"])


def module_arg(module):
    version = module.get("Version") or ""
    if version:
        return f'{module["Path"]}@{version}'
    return module["Path"]


def run(command, cwd, env=None, capture_output=False):
    try:
        return subprocess.run(
            command,
            check=True,
            cwd=cwd,
            env=env,
            text=True,
            capture_output=capture_output,
        )
    except subprocess.CalledProcessError as err:
        message = f"command failed: {shlex.join(command)}"
        stderr = (err.stderr or "").strip()
        if stderr:
            message = f"{message}\nstderr:\n{stderr}"
        raise RuntimeError(message) from err


def load_go_mod_json(repo_root, go_mod_path):
    # Use Go's own parser so the script never hand-parses go.mod syntax.
    result = run(
        ["go", "mod", "edit", "-json", str(go_mod_path)],
        cwd=repo_root,
        capture_output=True,
    )
    return json.loads(result.stdout)


def replacement_pair(replace):
    return module_arg(replace["Old"]), module_arg(replace["New"])


def root_require_versions(go_mod):
    return {
        require["Path"]: require["Version"]
        for require in go_mod.get("Require", [])
        if is_root_k8s_module_path(require["Path"])
    }


def fallback_k8s_version(root_replace_map, root_require_map):
    cloud_provider_replace = root_replace_map.get(K8S_CLOUD_PROVIDER_MODULE)
    if cloud_provider_replace is not None:
        version = cloud_provider_replace["New"].get("Version") or ""
        if version:
            return version

    version = root_require_map.get(K8S_CLOUD_PROVIDER_MODULE) or ""
    if version:
        return version

    raise ValueError(f"could not determine fallback version from {K8S_CLOUD_PROVIDER_MODULE}")


def target_replace_pairs(root, openshift_tests):
    root_replace_map = {
        replace["Old"]["Path"]: replace for replace in root.get("Replace", []) if is_root_k8s_replace(replace)
    }
    root_require_map = root_require_versions(root)
    openshift_tests_replace_map = {
        replace["Old"]["Path"]: replace
        for replace in openshift_tests.get("Replace", [])
        if is_root_k8s_replace(replace)
    }

    # Preserve any k8s.io replaces already present in openshift-tests, but
    # source their versions from the parent when available. Root-owned replaces
    # are always included so openshift-tests continues mirroring the parent set.
    target_paths = sorted(set(root_replace_map) | set(openshift_tests_replace_map))
    default_version = fallback_k8s_version(root_replace_map, root_require_map)

    target_pairs = []
    for path in target_paths:
        if path in root_replace_map:
            target_pairs.append(replacement_pair(root_replace_map[path]))
            continue

        version = root_require_map.get(path, default_version)
        target_pairs.append((path, f"{path}@{version}"))

    return target_pairs, sorted(replacement_pair(replace) for replace in openshift_tests_replace_map.values())


def build_edit_flags(root, openshift_tests):
    target_pairs, openshift_tests_pairs = target_replace_pairs(root, openshift_tests)

    # Compare normalized pairs first so reruns are a true no-op when the child
    # module already matches the parent policy.
    if target_pairs == openshift_tests_pairs:
        return []

    target_map = dict(target_pairs)
    openshift_tests_map = dict(openshift_tests_pairs)
    edit_flags = []

    # Update only the entries that actually changed. Recreating the full set
    # causes `go mod edit` to flatten the shared replace block in go.mod.
    for old in sorted(set(openshift_tests_map) - set(target_map)):
        edit_flags.append(f"-dropreplace={old}")

    for old in sorted(set(target_map) & set(openshift_tests_map)):
        if openshift_tests_map[old] != target_map[old]:
            edit_flags.append(f"-replace={old}={target_map[old]}")

    for old in sorted(set(target_map) - set(openshift_tests_map)):
        edit_flags.append(f"-replace={old}={target_map[old]}")

    return edit_flags


def main():
    repo_root = Path(__file__).resolve().parent.parent
    openshift_tests_dir = repo_root / "openshift-tests"
    openshift_tests_mod = openshift_tests_dir / "go.mod"

    root = load_go_mod_json(repo_root, repo_root / "go.mod")
    openshift_tests = load_go_mod_json(repo_root, openshift_tests_mod)
    edit_flags = build_edit_flags(root, openshift_tests)

    if not edit_flags:
        return 0

    run(["go", "mod", "edit", *edit_flags, str(openshift_tests_mod)], cwd=repo_root)

    return 0


if __name__ == "__main__":
    sys.exit(main())
