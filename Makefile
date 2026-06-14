APP_NAME := loinc-browser
DB ?= ./data/loinc-normalized.sqlite
ADDR ?= :8080
DEV_WEB_PORT ?= 5173
VERSION ?= dev
RELEASE ?= ./Loinc_2.82

.PHONY: help install web check test build serve dev dev-api dev-web ingest reingest release clean

help:
	@echo "Targets:"
	@echo "  make install          Install frontend dependencies"
	@echo "  make web              Build embedded Svelte assets"
	@echo "  make check            Run Go tests, Svelte check, and frontend build"
	@echo "  make build            Build local $(APP_NAME) binary"
	@echo "  make serve            Serve browser on ADDR=$(ADDR) with DB=$(DB)"
	@echo "  make dev              Run Go API reload watcher and Vite HMR server"
	@echo "  make dev-api          Run Go API with local source-change restart"
	@echo "  make dev-web          Run Vite HMR on DEV_WEB_PORT=$(DEV_WEB_PORT)"
	@echo "  make ingest RELEASE=./Loinc_2.82"
	@echo "  make reingest         Remove DB=$(DB), then ingest RELEASE=$(RELEASE)"
	@echo "  make release          Build macOS/Linux/Windows amd64 and arm packages"
	@echo "  make clean            Remove generated local build artifacts"

install:
	npm --prefix web install

web:
	npm --prefix web run build

test:
	go test ./...

check:
	npm --prefix web run build
	go test ./...
	npm --prefix web run check

build: web
	go build -trimpath -o ./$(APP_NAME) ./cmd/loinc-browser

serve: web
	go run ./cmd/loinc-browser serve --db $(DB) --addr $(ADDR)

dev:
	$(MAKE) -j2 dev-api dev-web

dev-api:
	DB=$(DB) ADDR=$(ADDR) ./scripts/dev-api.sh

dev-web:
	LOINC_API_TARGET=http://localhost$(ADDR) npm --prefix web run dev -- --host 0.0.0.0 --port $(DEV_WEB_PORT)

ingest:
	go run ./cmd/loinc-browser ingest --release $(RELEASE) --db $(DB)

reingest:
	rm -f $(DB) $(DB)-shm $(DB)-wal
	$(MAKE) ingest RELEASE=$(RELEASE) DB=$(DB)

release:
	VERSION=$(VERSION) ./scripts/build-release.sh

clean:
	rm -rf ./$(APP_NAME) ./dist
