.PHONY: build compose-up compose-down compose-logs fmt lint vet tidy testacc docs

VERSION ?= dev
TFPLUGINDOCS_VERSION ?= v0.25.0

build:
	go build -ldflags "-X main.version=$(VERSION)" -o terraform-provider-xyops .

docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION) generate --provider-name xyops --rendered-provider-name xyOps

compose-up:
	docker compose up -d

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f xyops

fmt:
	go run mvdan.cc/gofumpt@v0.12.0 -w .

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...

vet:
	go vet ./...

tidy:
	go mod tidy

testacc:
	TF_ACC=1 go test ./...
