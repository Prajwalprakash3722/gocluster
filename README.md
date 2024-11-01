# GoCluster

A Lightweight Distributed Cluster Manager in Go

[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

## Overview

GoCluster is a lightweight distributed cluster manager written in Go that simplifies cluster management through automatic node discovery, leader election, and state management. It's designed for small to medium-sized distributed systems, featuring an extensible operator plugin system for custom cluster operations.

Designed for SREs in mind, GoCluster automates repetitive cluster operations that would otherwise require manual intervention across multiple nodes. Instead of logging into each node to perform operations like creating namespaces or running backups, GoCluster provides a centralized way to manage these tasks across hundreds of clusters through simple operator definitions.

## Key Features

- **Node Management**
  - Automatic node discovery via UDP
  - Leader election based on node ID
  - Health monitoring and failover handling

- **Operator System**
  - Plugin-based architecture
  - Extensible framework for cluster tasks
  - Web interface integration

## Getting Started

### Prerequisites

- Go 1.21+
- Linux/Unix environment, if you have windows, please do a favor to yourself and throw it away:)

### Installation

```bash
git clone https://github.com/Prajwalprakash3722/gocluster
cd gocluster
make linux
```

### Configuration

Create `cluster.yaml`:
```yaml
cluster:
  name: mycluster
  discovery_port: 7946
  bind_address: 0.0.0.0
  web_address: 0.0.0.0:8080
  enable_operator: true

nodes:
  node001: node001:7946
  node002: node002:7946
  node003: node003:7946

plugins:
  - aerospike-config
```

### Usage

1. Start the agent:
```bash
./agent -c cluster.yaml
```

2. Access the web interface at `http://localhost:8080`, or use the CLI tool to manage the cluster. (web doesn't have any functionality to interact with the cluster yet, it can only show the status of the cluster and nodes)

### CLI Tool

The cluster can be managed using our [CLI tool](https://github.com/Prajwalprakash3722/gocluster-cli). Example commands:

```bash
# List available clusters
gocluster clusters

# Select a cluster
gocluster use stg-nodes

# List operators
gocluster operator list
```

## Operator System

GoCluster uses a plugin system for extending cluster functionality. Custom operators must implement:

```go
type Operator interface {
    Info() OperatorInfo
    Init(config map[string]interface{}) error
    Execute(ctx context.Context, params map[string]interface{}) error
    Rollback(ctx context.Context) error
    Cleanup() error
}
```

### Best Practices

- Implement proper error handling
- Use context for cancellation
- Provide rollback capabilities
- Include validation checks
- Maintain idempotency

## Project Structure

```
.
├── Dockerfile
├── LICENSE
├── Makefile
├── README.md
├── cluster.yaml
├── cmd
│   └── gocluster-manager
│       └── main.go
├── docker-compose.yml
├── go.mod
├── go.sum
├── gocluster-manager
├── gocluster-manager-macos
└── internal
    ├── cluster
    │   └── manager.go
    ├── config
    │   └── config.go
    ├── operator
    │   ├── manager.go
    │   └── plugins
    │       ├── aerospike
    │       │   ├── aerospike_operator.go
    │       │   └── config.go
    │       ├── hello
    │       │   └── operator.go
    │       └── mysql
    │           └── mysql_operator.go
    ├── types
    │   └── types.go
    └── web
        ├── handler.go
        └── templates
            └── index.html

14 directories, 21 files
```

## Roadmap
_(Highly dependent on my mood and time availability)_ :smile:
- [ ] Add Debug/Info/Error Logs (priority)
- [ ] Add Tests
- [x] Custom operation framework
- [ ] Distributed task execution system
- [ ] Secure communication (TLS/mTLS)
- [ ] Web-based cluster management UI
- [ ] Metrics and monitoring
- [ ] Multi-region support

## Contributing

Contributions are welcome. Please submit a pull request for review.

## License

[MIT License](LICENSE)

---
Created by [@prajwal.p](https://github.com/Prajwalprakash3722)
