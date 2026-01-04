.PHONY: build run test lint clean deploy deploy-prod logs status

build:
	go build -o bin/execbox-cloud ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

# Deployment
deploy:
	fly deploy

deploy-prod:
	fly deploy --app execbox-cloud

logs:
	fly logs

status:
	fly status
