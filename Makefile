.PHONY: build run clean

build:
	go build -o gocluster-manager cmd/agent/main.go

run: build
	./agent

clean:
	rm -f agent

linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "-w" -o gocluster-manager cmd/agent/main.go
