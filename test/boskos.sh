#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# acquires a project from boskos
acquire_project() {
    local project=""
    local project_type="gce-project"

    boskos_response=$(curl -X POST "http://boskos.test-pods.svc.cluster.local/acquire?type=${project_type}&state=free&dest=busy&owner=${JOB_NAME}")

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
    curl -X POST "http://boskos/release?name=${PROJECT}&owner=${JOB_NAME}&dest=dirty"
}

# send a heartbeat to boskos for the project
heartbeat_project() {
    curl -X POST "http://boskos/update?name=${PROJECT}&state=busy&owner=${JOB_NAME}" > /dev/null 2>&1
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
