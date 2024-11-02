FROM golang:1.21-alpine

WORKDIR /app

RUN apk add --no-cache git  #required for go mod download

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o gocluster-manager cmd/gocluster-manager/main.golinux

EXPOSE 8080 7946

ENTRYPOINT ["/app/gocluster-manager", "-c", "/app/cluster.yaml"]
