#!/bin/sh -ex

CNI_DIR="${CNI_DIR:-/host/opt/cni/bin}"
METIS_BINARY="/bin/metis"

echo "Installing metis CNI binary to ${CNI_DIR}..."
mkdir -p "${CNI_DIR}"
cp "${METIS_BINARY}" "${CNI_DIR}/metis"
chmod +x "${CNI_DIR}/metis"
echo "Metis CNI binary installed successfully to ${CNI_DIR}/metis"
ls -l "${CNI_DIR}/metis"
