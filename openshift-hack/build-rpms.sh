#!/bin/bash -x

source_path=_output/SOURCES

mkdir -p ${source_path}

# Getting version from args
version=${1:-4.14.0}

# Install dependencies required
dnf install -y rpmdevtools
dnf install -y createrepo
dnf builddep -y gcr-credential-provider.spec

# Use the build time as the release, ensuring we install this build.
release=${2:-$(date -u +'%Y%m%dT%H%M%SZ')}

# NOTE:        rpmbuild requires that inside the tar there will be a
#              ${service}-${version} directory, hence this --transform option.
#              We exclude .git as rpmbuild will do its own `git init`.
#              Excluding .tox is convenient for local builds.
tar -czf ${source_path}/gcr-credential-provider.tar.gz --exclude=.git --exclude=.tox --transform "flags=r;s|\.|gcr-credential-provider-${version}|" .


rpmbuild -ba -D "version $version" -D "release $release" -D "os_git_vars OS_GIT_VERSION=$version" -D "_topdir `pwd`/_output" gcr-credential-provider.spec

createrepo _output/RPMS/x86_64
