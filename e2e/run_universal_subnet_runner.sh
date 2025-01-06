#!/usr/bin/env sh

set -o errexit

git clone https://github.com/ConsiderItDone/universal-subnet-runner.git
cd ./universal-subnet-runner
# Build Subnet EVM, which is run as a subprocess
echo "Building Universal Subnet Runner"
echo "$GOPATH"
go version
grep "go ^[0-9.]+\n$" ./go.mod
go build -o ./universal-subnet-runner .

#./universal-subnet-runner --vm-name landslidevm --plugin-id pjSL9ksard4YE96omaiTkGL5H6XX2W5VEo3ZgWC9S2P6gzs9A
