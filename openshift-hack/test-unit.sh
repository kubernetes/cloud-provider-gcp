set -o errexit
set -o nounset
set -o pipefail

go test -count=1 -race -v $(go list ./...)
