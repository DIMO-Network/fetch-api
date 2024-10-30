.PHONY: clean run build install dep test lint format docker

PATHINSTBIN = $(abspath ./bin)
export PATH := $(PATHINSTBIN):$(PATH)
OLDSHELL := $(SHELL)
SHELL = env PATH=$(PATH) $(OLDSHELL)

BIN_NAME					?= fetch-api
DEFAULT_INSTALL_DIR			:= $(go env GOPATH)/$(PATHINSTBIN)
DEFAULT_ARCH				:= $(shell go env GOARCH)
DEFAULT_GOOS				:= $(shell go env GOOS)
ARCH						?= $(DEFAULT_ARCH)
GOOS						?= $(DEFAULT_GOOS)
INSTALL_DIR					?= $(DEFAULT_INSTALL_DIR)
.DEFAULT_GOAL 				:= run


# Dependency versions
GOLANGCI_VERSION   = latest
SWAGGO_VERSION     = $(shell go list -m -f '{{.Version}}' github.com/swaggo/swag)
PROTOC_VERSION             = 28.3
PROTOC_GEN_GO_VERSION      = $(shell go list -m -f '{{.Version}}' google.golang.org/protobuf) 
PROTOC_GEN_GO_GRPC_VERSION = $(shell go list -m -f '{{.Version}}' google.golang.org/grpc) 

help:
	@echo "\nSpecify a subcommand:\n"
	@grep -hE '^[0-9a-zA-Z_-]+:.*?## .*$$' ${MAKEFILE_LIST} | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[m %s\n", $$1, $$2}'
	@echo ""

build: ## Build the binary
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(ARCH) \
		go build -o $(PATHINSTBIN)/$(BIN_NAME) ./cmd/$(BIN_NAME)

run: build ## Run the binary
	@./$(PATHINSTBIN)/$(BIN_NAME)
all: clean target

clean: ## Clean all build artifacts
	@rm -rf $(PATHINSTBIN)
	
install: build ## Install the binary
	@install -d $(INSTALL_DIR)
	@rm -f $(INSTALL_DIR)/$(BIN_NAME)
	@cp $(PATHINSTBIN)/$(BIN_NAME) $(INSTALL_DIR)/$(BIN_NAME)

tidy: ## Tidy go modules
	@go mod tidy

test: ## Run tests
	@go test ./...

lint: ## Run linter
	@golangci-lint run

format: ## Format code
	@golangci-lint run --fix

docker: dep ## Build docker image
	@docker build -f ./Dockerfile . -t dimozone/$(BIN_NAME):$(VER_CUT)
	@docker tag dimozone/$(BIN_NAME):$(VER_CUT) dimozone/$(BIN_NAME):latest

tools-protoc:
	@mkdir -p $(PATHINSTBIN)
ifeq ($(shell uname | tr A-Z a-z), darwin)
	curl -L https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-osx-x86_64.zip > bin/protoc.zip
endif
ifeq ($(shell uname | tr A-Z a-z), linux)
	curl -L https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip > bin/protoc.zip
endif
	unzip -o $(PATHINSTBIN)/protoc.zip -d $(PATHINSTBIN)/protoclib 
	mv -f $(PATHINSTBIN)/protoclib/bin/protoc $(PATHINSTBIN)/protoc
	rm -rf $(PATHINSTBIN)/include
	mv $(PATHINSTBIN)/protoclib/include $(PATHINSTBIN)/ 
	rm $(PATHINSTBIN)/protoc.zip

tools-protoc-gen-go:
	@mkdir -p bin
	curl -L https://github.com/protocolbuffers/protobuf-go/releases/download/v${PROTOC_GEN_GO_VERSION}/protoc-gen-go.v${PROTOC_GEN_GO_VERSION}.$(shell uname | tr A-Z a-z).amd64.tar.gz | tar -zOxf - protoc-gen-go > ./bin/protoc-gen-go
	@chmod +x ./bin/protoc-gen-go

tools-protoc-gen-go-grpc:
	@mkdir -p bin
	curl -L https://github.com/grpc/grpc-go/releases/download/cmd/protoc-gen-go-grpc/v${PROTOC_GEN_GO_GRPC_VERSION}/protoc-gen-go-grpc.v${PROTOC_GEN_GO_GRPC_VERSION}.$(shell uname | tr A-Z a-z).amd64.tar.gz | tar -zOxf - ./protoc-gen-go-grpc > ./bin/protoc-gen-go-grpc
	@chmod +x ./bin/protoc-gen-go-grpc

tools-golangci-lint: ## Install golangci-lint
	@mkdir -p $(PATHINSTBIN)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | BINARY=golangci-lint bash -s -- ${GOLANGCI_VERSION}

tools-swagger: ## install swagger tool
	@mkdir -p $(PATHINSTBIN)
	GOBIN=$(PATHINSTBIN) go install github.com/swaggo/swag/cmd/swag@$(SWAGGO_VERSION)

tools: tools-golangci-lint tools-swagger tools-protoc tools-protoc-gen-go tools-protoc-gen-go-grpc  ## Install all tools

generate: go-generate generate-grpc generate-swagger ## run all file generation for the project

go-generate:## run go generate
	@go generate ./...

generate-grpc:
	@protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    pkg/grpc/*.proto

generate-swagger: ## generate swagger documentation
	@swag -version
	swag init -g cmd/fetch-api/main.go --parseDependency --parseInternal