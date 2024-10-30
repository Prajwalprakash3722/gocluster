<div align="center">

# 🌟 GoCluster

### A Lightweight Distributed Cluster Manager in Go

[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

</div>

---

## 📑 Overview

GoCluster is a lightweight distributed cluster manager written in Go that simplifies cluster management through automatic node discovery, leader election, and state management. It's designed for small to medium-sized distributed systems, featuring an extensible operator plugin system for custom cluster operations.

## ✨ Key Features

- **Node Management**
  - Node discovery via UDP
  - Smart leader election based on node ID
  - Real-time health monitoring
  - Automatic failover handling

- **Operator System**
  - Plugin-based architecture for custom operations
  - Extensible framework for cluster tasks
  - Web interface integration

- **Monitoring**
  - Real-time web interface
  - Cluster state visualization
  - Node health tracking
  - Operator status monitoring

## 🚀 Getting Started

### Prerequisites

```bash
- Go 1.21+
- Linux/Unix environment, if you have windows, please do a favor to yourself and throw it away:)
```

### Quick Start

1. **Installation**
```bash
git clone https://github.com/Prajwalprakash3722/gocluster
cd gocluster
make linux
```

2. **Configuration**
Create `cluster.yaml`:
```conf
# Cluster configuration
cluster:
  name: mycluster
  discovery_port: 7946
  bind_address: 0.0.0.0
  web_address: 0.0.0.0:8080
  enable_operator: true

# Node definitions
nodes:
  node001: node001:7946
  node002: node002:7946
  node003: node003:7946
plugins:
  - aerospike-config
```

3. **Run the Agent**
```bash
./agent -c cluster.conf
```

4. **Enable Web Interface**
```bash
./agent -c cluster.conf
```
Access at `http://localhost:8080`

## 🔌 Operator System

GoCluster features an rich operator plugin system for extending cluster functionality. Operators are modular components that can perform specific tasks across your cluster.

### Creating Custom Operators

All operators must implement the following interface:

```go
type Operator interface {
    Name() string
    Init(config map[string]interface{}) error
    Execute(ctx context.Context, params map[string]interface{}) error
    Rollback(ctx context.Context) error
    Cleanup() error
}
```
### Best Practices
- Implement proper error handling
- Use context for cancellation
- Provide rollback capabilities, very important incase of failure **(Hope is not a strategy)**.
- Include validation checks
- Maintain idempotency

The project includes an Aerospike operator as a reference implementation. This operator manages Aerospike database configuration across your cluster.


Place your operator in `internal/operator/plugins/youroperatorname/`

## 📊 Cluster Status Example

```text
Cluster Status:
Local node: node001 (State: leader)
Leader: node001
Cluster nodes:
- node002 (State: follower, Last seen: 1s)
- node003 (State: follower, Last seen: 1s)
```

## 📁 Project Structure

```
.
├── Dockerfile
├── LICENSE
├── Makefile
├── README.md
├── agent
├── cluster.yaml
├── cmd
│   └── agent
│       └── main.go
├── docker-compose.yml
├── go.mod
├── go.sum
├── gocluster-manager
└── internal
    ├── cluster
    │   ├── manager.go
    │   └── node.go
    ├── config
    │   └── config.go
    ├── operator
    │   ├── interface.go
    │   ├── manager.go
    │   └── plugins
    │       └── aerospike
    │           ├── config.go
    │           └── operator.go
    └── web
        ├── handler.go
        └── templates
            └── index.html

11 directories, 20 files
```

## 🛣️ Roadmap (Highly dependent on my mood and time availability)

- [x] Custom operation framework
- [ ] Distributed task execution system
- [ ] Secure communication (TLS/mTLS)
- [ ] Web-based cluster management UI
- [ ] Configuration replication
- [ ] Load balancing capabilities
- [ ] Metrics and monitoring
- [ ] Multi-region support

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## 📝 License

MIT License - see [LICENSE](LICENSE) for details.

<div align="center">
Crafted with ❤️ by <a href="https://github.com/Prajwalprakash3722">@prajwal.p</a>
</div>
