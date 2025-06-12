#!/bin/bash
# Simple ZooKeeper setup for testing with podman

echo "Starting ZooKeeper container for testing..."

podman run -d \
  --name zookeeper-test \
  --rm \
  -p 2181:2181 \
  -e ALLOW_ANONYMOUS_LOGIN=yes \
  docker.io/bitnami/zookeeper:3.8

echo "ZooKeeper is starting..."
echo "Client endpoint: localhost:2181"
echo "To stop: podman stop zookeeper-test"
echo "To check logs: podman logs zookeeper-test"
