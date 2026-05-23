K8S_NAMESPACE ?= banking
K8S_APP_PORT ?= 8080
K8S_DB_PORT ?= 15432
K8S_PROMETHEUS_PORT ?= 9090
K8S_GRAFANA_PORT ?= 3000
K8S_BASE_URL ?= http://localhost:$(K8S_APP_PORT)
K8S_DB_DSN ?= postgres://postgres:postgres@localhost:$(K8S_DB_PORT)/banking?sslmode=disable

.PHONY: dev lint fmt test build seed \
        up up-optimized down logs ps \
        k8s-up k8s-down k8s-status k8s-logs \
        k8s-port-forward k8s-port-forward-db \
        k8s-port-forward-prometheus k8s-port-forward-grafana \
        k8s-seed k8s-load-test

init:
	go mod download
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

dev:
	air

lint:
	golangci-lint-v2 run

test:
	go test -v ./...

build:
	go build -o bin/app cmd/server/main.go

ifeq ($(OS),Windows_NT) # Windows' command
seed:
	set DB_PRIMARY_DSN=postgres://postgres:postgres@localhost:5432/banking?sslmode=disable&& go run ./cmd/seeds/main.go
else
seed:
	DB_PRIMARY_DSN=postgres://postgres:postgres@localhost:5432/banking?sslmode=disable go run ./cmd/seeds/main.go
endif

up:
	cp .env.baseline.example .env
	docker compose up -d --build

up-optimized:
	cp .env.optimized.example .env
	docker compose --profile optimized up -d --build

down:
	docker compose --profile optimized down

load-test:
	k6 run scripts/load-test/mixed.js

logs:
	docker compose logs -f

ps:
	docker compose ps

k8s-up:
	kubectl apply -f deployments/k8s/namespace.yaml
	kubectl apply -f deployments/k8s/

k8s-down:
	kubectl delete -f deployments/k8s/ --ignore-not-found
	kubectl delete -f deployments/k8s/namespace.yaml --ignore-not-found

k8s-status:
	kubectl -n $(K8S_NAMESPACE) get pods,svc,hpa

k8s-logs:
	kubectl -n $(K8S_NAMESPACE) logs -f deploy/banking-app

k8s-port-forward:
	kubectl -n $(K8S_NAMESPACE) port-forward svc/banking-app $(K8S_APP_PORT):8080

k8s-port-forward-db:
	kubectl -n $(K8S_NAMESPACE) port-forward svc/postgres $(K8S_DB_PORT):5432

k8s-port-forward-prometheus:
	kubectl -n $(K8S_NAMESPACE) port-forward svc/prometheus $(K8S_PROMETHEUS_PORT):9090

k8s-port-forward-grafana:
	kubectl -n $(K8S_NAMESPACE) port-forward svc/grafana $(K8S_GRAFANA_PORT):3000

k8s-seed:
	DB_PRIMARY_DSN=$(K8S_DB_DSN) go run ./cmd/seeds/main.go

k8s-load-test:
	BASE_URL=$(K8S_BASE_URL) k6 run scripts/load-test/mixed.js
