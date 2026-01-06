#!/bin/bash
set -e

cd "$(dirname "$0")"

# Create output directory
mkdir -p src/generated

# Generate OpenAPI spec from the Go server and save it to the repo root
echo "Generating OpenAPI spec..."
go run ../cmd/server/main.go --openapi > ../openapi.json

# Generate TypeScript types from the spec
echo "Generating TypeScript types..."
npx openapi-typescript ../openapi.json -o src/generated/api.ts

echo "TypeScript types generated successfully"
