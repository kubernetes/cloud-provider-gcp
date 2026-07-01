import importlib.util
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("sync-openshift-tests-k8s-deps.py")
SPEC = importlib.util.spec_from_file_location("sync_openshift_tests_k8s_deps", SCRIPT_PATH)
SYNC = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(SYNC)


def require(path, version):
    return {"Path": path, "Version": version}


def replace(old_path, version, new_path=None):
    new = {"Path": new_path or old_path}
    if version:
        new["Version"] = version
    return {"Old": {"Path": old_path}, "New": new}


class SyncPolicyTest(unittest.TestCase):
    def test_target_replace_pairs_prefer_root_replace_then_require_then_fallback(self):
        root = {
            "Require": [
                require("k8s.io/cloud-provider", "v1.31.1"),
                require("k8s.io/api", "v1.31.2"),
                require("k8s.io/apimachinery", "v1.31.3"),
            ],
            "Replace": [
                replace("k8s.io/apimachinery", "v1.31.99"),
            ],
        }
        openshift_tests = {
            "Replace": [
                replace("k8s.io/apimachinery", "v0.0.1"),
                replace("k8s.io/api", "v0.0.2"),
                replace("k8s.io/client-go", "v0.0.3"),
            ],
        }

        target_pairs, openshift_tests_pairs = SYNC.target_replace_pairs(root, openshift_tests)

        self.assertEqual(
            [
                ("k8s.io/api", "k8s.io/api@v1.31.2"),
                ("k8s.io/apimachinery", "k8s.io/apimachinery@v1.31.99"),
                ("k8s.io/client-go", "k8s.io/client-go@v1.31.1"),
            ],
            target_pairs,
        )
        self.assertEqual(
            [
                ("k8s.io/api", "k8s.io/api@v0.0.2"),
                ("k8s.io/apimachinery", "k8s.io/apimachinery@v0.0.1"),
                ("k8s.io/client-go", "k8s.io/client-go@v0.0.3"),
            ],
            openshift_tests_pairs,
        )

    def test_build_edit_flags_is_noop_when_pairs_already_match(self):
        root = {
            "Require": [
                require("k8s.io/cloud-provider", "v1.31.1"),
                require("k8s.io/api", "v1.31.2"),
            ],
            "Replace": [
                replace("k8s.io/apimachinery", "v1.31.99"),
            ],
        }
        openshift_tests = {
            "Replace": [
                replace("k8s.io/api", "v1.31.2"),
                replace("k8s.io/apimachinery", "v1.31.99"),
                replace("k8s.io/client-go", "v1.31.1"),
            ],
        }

        self.assertEqual([], SYNC.build_edit_flags(root, openshift_tests))

    def test_build_edit_flags_only_updates_changed_paths(self):
        root = {
            "Require": [
                require("k8s.io/cloud-provider", "v1.31.1"),
                require("k8s.io/api", "v1.31.2"),
            ],
            "Replace": [
                replace("k8s.io/apimachinery", "v1.31.99"),
                replace("k8s.io/client-go", "v1.31.4"),
            ],
        }
        openshift_tests = {
            "Replace": [
                replace("k8s.io/api", "v0.0.2"),
                replace("k8s.io/apimachinery", "v1.31.99"),
                replace("k8s.io/kubelet", "v0.0.3"),
            ],
        }

        self.assertEqual(
            [
                "-replace=k8s.io/api=k8s.io/api@v1.31.2",
                "-replace=k8s.io/kubelet=k8s.io/kubelet@v1.31.1",
                "-replace=k8s.io/client-go=k8s.io/client-go@v1.31.4",
            ],
            SYNC.build_edit_flags(root, openshift_tests),
        )


if __name__ == "__main__":
    unittest.main()
