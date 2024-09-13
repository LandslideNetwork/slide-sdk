#!/usr/bin/env bash

set -e

#################################
#set default values
ANR_PORT=8080
ANR_REPO_PATH=github.com/ava-labs/avalanche-network-runner
ANR_VERSION=v1.7.7

#################################
# download avalanche-network-runner
go install -v ${ANR_REPO_PATH}@${ANR_VERSION}

#################################
# run "avalanche-network-runner" server
GOPATH=$(go env GOPATH)

echo "launch avalanche-network-runner in the background"
${GOPATH}/bin/avalanche-network-runner server \
  --log-level debug \
  --port=":${ANR_PORT}" \
  --disable-grpc-gateway &
PID=${!}

echo "network-runner RPC server is running on PID ${PID}..."
echo ""
echo "use the following command to terminate:"
echo ""
echo "pkill -P ${PID} && kill -2 ${PID} && pkill -9 -f tGas3T58KzdjLHhBDMnH2TvrddhqTji5iZAMZ3RXs2NLpSnhH"
echo ""

#################################
# start the landslide network
${GOPATH}/bin/avalanche-network-runner control start --log-level debug --endpoint="0.0.0.0:$ANR_PORT" \
 --blockchain-specs "[{\"vm_name\":\"landslidevm\",\"genesis\":\"${LANDSLIDE_GENESIS_PATH}\",\"blockchain_alias\":\"landslidesdk\"}]" \
  --avalanchego-path "${AVALANCHEGO_PATH}" --plugin-dir "${AVALANCHEGO_PLUGINS_PATH}"
