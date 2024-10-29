<div align="center">

# 🌟 GoCluster

### A Lightweight Distributed Cluster Manager in Go

[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

</div>

---

## 🎯 Overview

GoCluster is a lightweight distributed cluster manager written in Go that provides automatic node discovery, leader election, and robust cluster state management. Perfect for small to medium-sized distributed systems that need simple yet effective cluster management.

## 🚀 Features

- **Automatic Node Discovery** - Node discovery using UDP
- **Smart Leader Election** - Simple and efficient leader election based on node ID
- **Health Monitoring** - Continuous health checks with automatic failover
- **State Management** - Clean and consistent cluster state management
- **Simple Configuration** - Easy setup using standard configuration files
- **Web Interface** - Real-time cluster monitoring with automatic updates and interactive displays (see below for setup)


## 🛠️ Getting Started

### Prerequisites

- Go 1.21 or higher
- Linux/Unix environment, If you have windows, throw it out :)

### Installation

```bash
git clone https://github.com/Prajwalprakash3722/gocluster
cd gocluster
make linux
```

### Configuration

1. Create your `cluster.conf` file:

```conf
[cluster]
name = proxy
discovery_port = 7946

[nodes]
node001 = "node001.devcoffee.me:7946"
node002 = "node002.devcoffee.me:7946"
node003 = "node003.devcoffee.me:7946"
```

2. Run the agent:

```bash
./agent -config cluster.conf -bind-address 0.0.0.0 -port 7946
```
3. Enabling the Web Interface
To view the real-time cluster monitoring interface:

Start the agent with the web server enabled by adding the -web flag:

```bash
./agent -config cluster.conf -bind-address 0.0.0.0 -port 7946 -web 0.0.0.0:8080
```
Access the web UI at http://node001:8080. (The interface provides an interactive overview of your cluster, including each node’s status, leader election details, and last seen timestamps.)


## 📊 Sample Outputs

### Initial Discovery
```text
Cluster Status:
Local node: node001 (State: leader)
Leader: node001
Cluster nodes:
- node003.devcoffee.me (State: follower, Last seen: 1s)
- node004.devcoffee.me (State: follower, Last seen: 1s)
- node002.devcoffee.me (State: follower, Last seen: 1s)
```

### Failover Scenario
```text
Cluster Status:
Local node: node003 (State: follower)
Leader: node001
Cluster nodes:
- node004.devcoffee.me (State: follower, Last seen: 0s)
- node001.devcoffee.me (State: leader, Last seen: 6s)
- node002.devcoffee.me (State: follower, Last seen: 0s)

2024/10/29 12:35:52 Node timeout: node001
2024/10/29 12:35:52 Leader node001 timed out, initiating new election
2024/10/29 12:35:52 Following new leader: node002
```

### Auto-Discovery
```text
2024/10/29 12:36:50 New node discovered: node001
2024/10/29 12:36:50 Following new leader: node001
2024/10/29 12:36:50 Updated node node004 state to follower
2024/10/29 12:36:50 Updated node node002 state to follower
```

## 🗺️ Future Plans (mainly dependent on my interest)

- [ ] Distributed task execution system
- [ ] Secure communication (TLS/mTLS)
- [ ] Web-based cluster management UI
- [ ] Custom operation framework
- [ ] Configuration replication
- [ ] Load balancing capabilities
- [ ] Metrics and monitoring
- [ ] Multi-region support
- [ ] Custom plugin system

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🌴 Project Tree
```bash
.
├── Dockerfile
├── Makefile
├── README.md
├── cluster.conf
├── cmd
│   └── agent
│       └── main.go
├── docker-compose.yml
├── go.mod
└── internal
    ├── cluster
    │   ├── manager.go
    │   └── node.go
    └── config
        └── config.go

6 directories, 10 files
```

---

<div align="center">
Made with ❤️ by <a href="https://github.com/Prajwalprakash3722">@prajwal.p</a>
</div>