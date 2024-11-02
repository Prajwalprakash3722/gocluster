# Build targets
.PHONY: build run clean linux docker-build docker-up docker-down docker-logs test config

# Variables
BINARY_MACOS=gocluster-manager-macos
BINARY_LINUX=gocluster-manager
CONFIG_FILE=docker/cluster.yaml

# Build for macOS
build:
	go build -o ${BINARY_MACOS} cmd/gocluster-manager/main.go

# Build for Linux
linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "-w" -o ${BINARY_LINUX} cmd/gocluster-manager/main.go

# Run locally on macOS
run: build
	./${BINARY_MACOS} -c ${CONFIG_FILE}

# Clean up binaries
clean:
	rm -f ${BINARY_MACOS}
	rm -f ${BINARY_LINUX}
	rm -f cluster*.yaml
	podman-compose down -v || true

# Create configuration files
config:
	@echo "Creating Test config files..."
	@cat << 'EOF' > docker/cluster.yaml
	cluster:
	  name: local
	  discovery_port: 7946
	  bind_address: 0.0.0.0
	  web_address: 0.0.0.0:8080
	  enable_operators: true
	nodes:
	  node2: node2:7946
	  node3: node3:7946
	plugins:
	  - hello
	EOF
	@cat << 'EOF' > docker/cluster2.yaml
	cluster:
	  name: local
	  discovery_port: 7946
	  bind_address: 0.0.0.0
	  web_address: 0.0.0.0:8080
	  enable_operators: true
	nodes:
	  node1: node1:7946
	  node3: node3:7946
	plugins:
	  - hello
	EOF
	@cat << 'EOF' > docker/cluster3.yaml
	cluster:
	  name: local
	  discovery_port: 7946
	  bind_address: 0.0.0.0
	  web_address: 0.0.0.0:8080
	  enable_operators: true
	nodes:
	  node1: node1:7946
	  node2: node2:7946
	plugins:
	  - hello
	EOF
	@echo "Test Config files created successfully"

# Docker/Podman targets
docker-build: linux
	podman build -t gocluster-manager .

docker-up: config docker-build
	podman-compose up -d

docker-down:
	podman-compose down

docker-logs:
	podman-compose logs -f

test-hello:
	curl -X POST http://localhost:8081/api/operator/hello \
		-H "Content-Type: application/json" \
		-d '{"operation":"greet","params":{"name":"world"},"parallel":true}'

# Development helpers
dev: config run

# Full cluster deployment
deploy: docker-up
	@echo "Waiting for cluster to start..."
	@sleep 10 # needs 5-10s to start the containers
	@make test-cluster

# Show running status
status:
	@echo "=== Cluster Status ==="
	@podman-compose ps
	@echo "\n=== Container Logs ==="
	@podman-compose logs --tail=20
