.PHONY: build test test-integration proto docker-build docker-up bench mem-compare run clean

build:
	go build -o bin/aegis ./cmd/aegis

run: build
	./bin/aegis -config config.yaml

test:
	go test ./... -short

test-integration:
	go test ./test/integration/... -v

proto:
	protoc --go_out=. --go_opt=module=aegis \
	       --go-grpc_out=. --go-grpc_opt=module=aegis \
	       api/proto/ratelimit.proto

docker-build:
	docker build -f deployments/docker/Dockerfile -t aegis:latest .

docker-up:
	cd deployments/docker && docker compose up --build

bench:
	k6 run scripts/benchmark/mixed_workload.js

mem-compare:
	go run scripts/memory/compare_memory_usage.go

clean:
	rm -rf bin/
