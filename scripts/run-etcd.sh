#!/bin/bash
# Simple etcd setup for testing with podman

echo "Starting etcd container for testing..."

podman run -d \
  --name etcd-test \
  --rm \
  -p 2379:2379 \
  -p 2380:2380 \
  quay.io/coreos/etcd:v3.5.9 \
  /usr/local/bin/etcd \
  --name etcd0 \
  --data-dir /etcd-data \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://localhost:2379 \
  --listen-peer-urls http://0.0.0.0:2380 \
  --initial-advertise-peer-urls http://localhost:2380 \
  --initial-cluster etcd0=http://localhost:2380 \
  --initial-cluster-token etcd-cluster-1 \
  --initial-cluster-state new \
  --log-level info \
  --logger zap \
  --log-outputs stderr

echo "etcd is starting..."
echo "Client endpoint: http://localhost:2379"
echo "To stop: podman stop etcd-test"
echo "To check logs: podman logs etcd-test"
