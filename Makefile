.PHONY: build test lint clean run docker-build

BINARY_NAME=k6-exporter
DOCKER_IMAGE=k6-prometheus-exporter
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.version=${VERSION}"

build:
	go build ${LDFLAGS} -o ${BINARY_NAME} ./cmd/exporter

test:
	go test -v -race ./...

lint:
	golangci-lint run

clean:
	rm -f ${BINARY_NAME}

run: build
	./${BINARY_NAME}

docker-build:
	docker build -t ${DOCKER_IMAGE}:${VERSION} -t ${DOCKER_IMAGE}:latest -f deployments/docker/Dockerfile .

docker-run: docker-build
	docker run -p 9090:9090 -e K6_API_TOKEN=${K6_API_TOKEN} ${DOCKER_IMAGE}:latest

deps:
	go mod download
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...