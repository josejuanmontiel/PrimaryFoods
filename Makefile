.PHONY: build run test clean export import-bedca

build:
	go build -o bin/primaryfoods ./cmd/server

run:
	go run ./cmd/server -port 8086 -db data/primaryfoods.db

test:
	go test -v ./...

import-bedca:
	go run ./cmd/importer -db data/primaryfoods.db

export:
	curl -s http://localhost:8086/api/export/json > primary_foods_export.json

clean:
	rm -rf bin/ data/ primary_foods_export.json
