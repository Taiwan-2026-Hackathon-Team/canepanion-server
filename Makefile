run:
	go run cmd/main.go

migrate:
	go run cmd/main.go --migrate

test:
	go test ./...

fmt:
	gofmt -w .

dev:
	air -c air.toml

ingest:
	go run ./cmd/caneingest
