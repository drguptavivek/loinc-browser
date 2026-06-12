APP_NAME := loinc-browser
DB ?= ./data/loinc.sqlite
ADDR ?= :8080
VERSION ?= dev

.PHONY: help install web check test build serve ingest release clean

help:
	@echo "Targets:"
	@echo "  make install          Install frontend dependencies"
	@echo "  make web              Build embedded Svelte assets"
	@echo "  make check            Run Go tests, Svelte check, and frontend build"
	@echo "  make build            Build local $(APP_NAME) binary"
	@echo "  make serve            Serve browser on ADDR=$(ADDR) with DB=$(DB)"
	@echo "  make ingest RELEASE=./Loinc_2.82"
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

ingest:
	@test -n "$(RELEASE)" || (echo "Set RELEASE=./Loinc_2.82" && exit 1)
	go run ./cmd/loinc-browser ingest --release $(RELEASE) --db $(DB)

release:
	VERSION=$(VERSION) ./scripts/build-release.sh

clean:
	rm -rf ./$(APP_NAME) ./dist
