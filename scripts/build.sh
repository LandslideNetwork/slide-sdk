#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

go_version_minimum="1.20.12"

go_version() {
    go version | sed -nE -e 's/[^0-9.]+([0-9.]+).+/\1/p'
}

version_lt() {
    # Return true if $1 is a lower version than than $2,
    local ver1=$1
    local ver2=$2
    # Reverse sort the versions, if the 1st item != ver1 then ver1 < ver2
    if [[ $(echo -e -n "$ver1\n$ver2\n" | sort -rV | head -n1) != "$ver1" ]]; then
        return 0
    else
        return 1
    fi
}

if version_lt "$(go_version)" "$go_version_minimum"; then
    echo "LandslideVM requires Go >= $go_version_minimum, Go $(go_version) found." >&2
    exit 1
fi

# Root directory
SUBNET_EVM_PATH=$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    cd .. && pwd
)

# Load the versions
source "$SUBNET_EVM_PATH"/scripts/versions.sh

# Load the constants
source "$SUBNET_EVM_PATH"/scripts/constants.sh

if [[ $# -eq 1 ]]; then
    BINARY_PATH=$1
elif [[ $# -eq 0 ]]; then
    BINARY_PATH="$GOPATH/src/github.com/ava-labs/avalanchego/build/plugins/pjSL9ksard4YE96omaiTkGL5H6XX2W5VEo3ZgWC9S2P6gzs9A"
else
    echo "Invalid arguments to build subnet-evm. Requires zero (default location) or one argument to specify binary location."
    exit 1
fi

# Build Subnet EVM, which is run as a subprocess
echo "Building Landslide VM at $BINARY_PATH"
go build -o "$BINARY_PATH" "example/countervm/"*.go
