#!/bin/bash

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 filename"
    exit 1
fi

filename=$1

goland /tmp/e2e-test-landslide/nodes/node1/logs/$filename
goland /tmp/e2e-test-landslide/nodes/node2/logs/$filename
goland /tmp/e2e-test-landslide/nodes/node3/logs/$filename
goland /tmp/e2e-test-landslide/nodes/node4/logs/$filename
goland /tmp/e2e-test-landslide/nodes/node5/logs/$filename
