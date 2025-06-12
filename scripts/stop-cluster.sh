#!/bin/bash
# Script to stop the test cluster

echo "🛑 Stopping gocluster test cluster..."

# Stop processes if PID files exist
if [ -f node1.pid ]; then
    NODE1_PID=$(cat node1.pid)
    if kill -0 $NODE1_PID 2>/dev/null; then
        kill $NODE1_PID
        echo "Stopped Node 1 (PID: $NODE1_PID)"
    fi
    rm -f node1.pid
fi

if [ -f node2.pid ]; then
    NODE2_PID=$(cat node2.pid)
    if kill -0 $NODE2_PID 2>/dev/null; then
        kill $NODE2_PID
        echo "Stopped Node 2 (PID: $NODE2_PID)"
    fi
    rm -f node2.pid
fi

if [ -f node3.pid ]; then
    NODE3_PID=$(cat node3.pid)
    if kill -0 $NODE3_PID 2>/dev/null; then
        kill $NODE3_PID
        echo "Stopped Node 3 (PID: $NODE3_PID)"
    fi
    rm -f node3.pid
fi

# Also try to kill any remaining gocluster processes
pkill -f "gocluster-manager-zk"

echo "✅ All nodes stopped"
echo ""
echo "📋 Log files are preserved:"
echo "  node1.log, node2.log, node3.log"
echo ""
echo "🗑️  To clean up logs:"
echo "  rm -f node*.log"
