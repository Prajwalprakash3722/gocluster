#!/bin/bash
# Script to test cluster operations

echo "🧪 GoCluster Operations Test"
echo "============================"

# Test hello operator
echo "🔧 Testing hello operator..."
echo "Sending greet operation to Node 1..."
curl -X POST http://localhost:8080/api/operator/trigger/hello \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "greet",
    "params": {"name": "World", "custom_message": "Hello from ZooKeeper cluster!"},
    "config": {}
  }' | jq .

echo ""

# Test jobs operator - validate job
echo "🔧 Testing jobs operator (validate)..."
echo "Sending validate_job operation to Node 2..."
curl -X POST http://localhost:8081/api/operator/trigger/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "validate_job",
    "params": {
      "job_yaml": "name: test-job\ndescription: A simple test job\nsteps:\n  - name: Hello Step\n    script: echo \"Hello from test job!\""
    },
    "config": {}
  }' | jq .

echo ""

# Test jobs operator - list logs
echo "🔧 Testing jobs operator (list logs)..."
echo "Sending list_logs operation to Node 3..."
curl -X POST http://localhost:8082/api/operator/trigger/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "list_logs",
    "params": {"limit": 10},
    "config": {}
  }' | jq .

echo ""

# Execute a simple job
echo "🔧 Testing jobs operator (execute job)..."
echo "Sending execute_job operation to Node 1..."
curl -X POST http://localhost:8080/api/operator/trigger/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "execute_job",
    "params": {
      "job_yaml": "name: cluster-test\ndescription: Test job for cluster\nsteps:\n  - name: System Info\n    script: |\n      echo \"=== Cluster Test ===\"\n      echo \"Hostname: $(hostname)\"\n      echo \"Date: $(date)\"\n      echo \"Uptime: $(uptime)\"\n  - name: Create Test File\n    script: |\n      echo \"Test from gocluster-zk\" > /tmp/cluster-test.txt\n      echo \"File created: $(ls -la /tmp/cluster-test.txt)\""
    },
    "config": {
      "working_dir": "/tmp",
      "timeout": "5m",
      "enable_logging": true
    }
  }' | jq .

echo ""
echo "🎉 Test operations completed!"
echo ""
echo "📋 Check logs with:"
echo "  tail -f node1.log"
echo "  tail -f node2.log" 
echo "  tail -f node3.log"
