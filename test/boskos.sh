#!/usr/bin/env bash

# Copyright 2024 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Source this file to be able to acquire a boskos project using
# the acquire_project function. Must be run in prow, since acquiring
# a boskos project expects the owner to be the prow job name (JOB_NAME).
# Can set a custom boskos url by passing the BOSKOS_URL env var.

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

if [[ -z "${BOSKOS_URL:-}" ]]; then
  BOSKOS_URL="http://boskos.test-pods.svc.cluster.local"
fi

# Acquires a "gce" project from boskos. Returns project by setting/exporting PROJECT env var.
# Parameter: JOB_NAME is an env var set by prow. Parameter: BOSKOS_URL is an env var set above
# by either passing in the env var, or using the default url. Starts/runs a heartbeat with
# the returned bosko project. Returns an error if unable to acquire the boskos project.
acquire_project() {
    local project=""
    local project_type="gce-project"

    boskos_response=$(curl -X POST "${BOSKOS_URL}/acquire?type=${project_type}&state=free&dest=busy&owner=${JOB_NAME}")
    echo
    echo "DEBUG--Boskos Response: ${boskos_response}"
    echo
    if project=$(echo "${boskos_response}" | jq -r '.name'); then
        echo "Using GCP project: ${project}"
        PROJECT="${project}"
        export PROJECT
        heartbeat_project_forever &
        BOSKOS_HEARTBEAT_PID=$!
        export BOSKOS_HEARTBEAT_PID
    else
        (>&2 echo "ERROR: failed to acquire GCP project. boskos response was: ${boskos_response}")
        exit 1
    fi
}

# release the project back to boskos
release_project() {
    curl -X POST "${BOSKOS_URL}/release?name=${PROJECT}&owner=${JOB_NAME}&dest=dirty"
}

# send a heartbeat to boskos for the project
heartbeat_project() {
    curl -X POST "${BOSKOS_URL}/update?name=${PROJECT}&state=busy&owner=${JOB_NAME}" > /dev/null 2>&1
}

# heartbeat_project in an infinite loop
heartbeat_project_forever() {
    set +x;
    local heartbeat_seconds=10
    while :
    do
        # always heartbeat, ignore failures
        heartbeat_project || true
        sleep ${heartbeat_seconds}
    done
}

cleanup_boskos () {
    # stop heartbeating
    kill "${BOSKOS_HEARTBEAT_PID}" || true
    # mark the project as dirty
    release_project
}
