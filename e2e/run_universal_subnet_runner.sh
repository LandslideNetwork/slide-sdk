#!/usr/bin/env sh

set -o errexit

TARGET_DIR="./universal-subnet-runner"
REPO_URL="https://github.com/ConsiderItDone/universal-subnet-runner.git"

# Check if the directory exists
if [ -d "$TARGET_DIR" ]; then
    echo "Directory $TARGET_DIR already exists."
else
    echo "Directory $TARGET_DIR does not exist. Cloning repository..."
    git clone "$REPO_URL" "$TARGET_DIR"
    if [ $? -eq 0 ]; then
        echo "Repository cloned successfully."
    else
        echo "Failed to clone repository." >&2
        exit 1
    fi
fi

cd ./universal-subnet-runner
# Build Subnet EVM, which is run as a subprocess
echo "Building Universal Subnet Runner"
export PATH=$HOME/sdk/go1.22.10/bin:$PATH
go version
go build -o ./universal-subnet-runner .


cp ../genesis ./data/
./universal-subnet-runner --vm-name landslidevm --plugin-id pjSL9ksard4YE96omaiTkGL5H6XX2W5VEo3ZgWC9S2P6gzs9A
