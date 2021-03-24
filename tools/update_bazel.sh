#!/bin/bash
set -o xtrace
set -o errexit
set -o nounset
set -o pipefail

bazel run //:gazelle
