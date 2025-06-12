#!/bin/bash
# Script to start 3-node gocluster test cluster with ZooKeeper backend

echo "🚀 Starting 3-node gocluster test cluster with ZooKeeper backend"
echo "==============================================================="

# Check if ZooKeeper is running
if ! podman ps | grep -q zookeeper-test; then
    echo "❌ ZooKeeper is not running. Please start it first:"
    echo "   ./run-zookeeper.sh"
    exit 1
fi

echo "✅ ZooKeeper is running"

# Build the latest version
echo "🔨 Building gocluster-manager..."
go build -o gocluster-manager-zk ./cmd/gocluster-manager

# Start node 1
echo "🟢 Starting Node 1 (port 8080)..."
./gocluster-manager-zk -c node1-zk-config.yaml > node1.log 2>&1 &
NODE1_PID=$!
echo "Node 1 PID: $NODE1_PID"

# Wait a bit for node 1 to start
sleep 3

# Start node 2
echo "🟡 Starting Node 2 (port 8081)..."
./gocluster-manager-zk -c node2-zk-config.yaml > node2.log 2>&1 &
NODE2_PID=$!
echo "Node 2 PID: $NODE2_PID"

# Wait a bit for node 2 to start
sleep 3

# Start node 3
echo "🔵 Starting Node 3 (port 8082)..."
./gocluster-manager-zk -c node3-zk-config.yaml > node3.log 2>&1 &
NODE3_PID=$!
echo "Node 3 PID: $NODE3_PID"

# Save PIDs for easy cleanup
echo "$NODE1_PID" > node1.pid
echo "$NODE2_PID" > node2.pid
echo "$NODE3_PID" > node3.pid

echo ""
echo "🎉 All nodes started! Web interfaces:"
echo "  Node 1: http://localhost:8080"
echo "  Node 2: http://localhost:8081" 
echo "  Node 3: http://localhost:8082"
echo ""
echo "📊 To check cluster status:"
echo "  curl http://localhost:8080/api/status"
echo ""
echo "📋 To view logs:"
echo "  tail -f node1.log  # Node 1 logs"
echo "  tail -f node2.log  # Node 2 logs"
echo "  tail -f node3.log  # Node 3 logs"
echo ""
echo "🛑 To stop all nodes:"
echo "  ./stop-cluster.sh"
