#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

KUBERNETES_LKG_PATTERN="k8s-lkg-update-*"
NEWEST_LKG_BRANCH=$(git for-each-ref --sort=committerdate refs/heads/ --format='%(refname:short)' | grep $KUBERNETES_LKG_PATTERN | head -n1)
git checkout master
git merge $NEWEST_LKG_BRANCH
git push origin master

git for-each-ref --sort=committerdate refs/heads/ --format='%(refname:short)' | grep $KUBERNETES_LKG_PATTERN | tail -n +1 | while read -r $OLD_LKG_BRANCH ; do
    git push -d origin $OLD_LKG_BRANCH
done