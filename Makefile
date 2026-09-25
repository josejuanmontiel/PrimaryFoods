.PHONY: build run test clean export import-bedca import-recipes import-off analyze

build:
	go build -o bin/primaryfoods ./cmd/server
	go build -o bin/analyzer ./cmd/analyzer
	go build -o bin/importer ./cmd/importer

run:
	go run ./cmd/server -port 8086 -db data/primaryfoods.db

test:
	go test -v ./...

import-bedca:
	go run ./cmd/importer -db data/primaryfoods.db -bedca

import-recipes:
	go run ./cmd/importer -db data/primaryfoods.db -recipes data/qwen_recipes.json

import-off:
	go run ./cmd/importer -db data/primaryfoods.db -off /home/jose/workspace/josejuanmontiel/buscatureceta/src/public/spain_products.tsv.zz -off-limit 50000

index-bases:
	go run ./cmd/importer -db data/primaryfoods.db -index-bases

analyze:
	go run ./cmd/analyzer -db data/primaryfoods.db -recipes data/qwen_recipes.json

export:
	curl -s http://localhost:8086/api/export/json > primary_foods_export.json

export-buscatureceta:
	curl -s http://localhost:8086/api/export/buscatureceta > data/buscatureceta_recipes_enriched.json
	@echo "Recetas enriquecidas exportadas a data/buscatureceta_recipes_enriched.json"

openapi:
	python3 -c "import yaml, json; data = yaml.safe_load(open('api/openapi.yaml')); json.dump(data, open('api/openapi.json', 'w'), indent=2); json.dump(data, open('internal/api/web/openapi.json', 'w'), indent=2); print('OpenAPI 3.1 JSON generated successfully')"
	cp api/openapi.yaml internal/api/web/openapi.yaml

clean:
	rm -rf bin/ primary_foods_export.json


