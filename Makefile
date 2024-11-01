.PHONY: build run clean

build:
	go build -o gocluster-manager-macos cmd/gocluster-manager/main.go

run: build
	./gocluster-manager-macos

clean:
	rm -f gocluster-manager-macos
	rm -f gocluster-manager

linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "-w" -o gocluster-manager cmd/gocluster-manager/main.go
