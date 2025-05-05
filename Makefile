.PHONY: all
all: build-binary build-container

GOLANGCI_LINT_VERSION=v2.1.6
GO_BINARY?=go

# the fallback '|| echo "golangci-lint' really expects this file
# NOT to exist! This is just a trigger to help installing golangci-lint
GOLANGCI_LINT_BIN=$(shell which golangci-lint 2>/dev/null || echo "golangci-lint")

.PHONY: help
help:
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@awk 'match($$0, /^([a-zA-Z_\/-]+):.*?## (.*)$$/, m) {printf "  \033[36m%-30s\033[0m %s\n", m[1], m[2]}' $(MAKEFILE_LIST) | sort

.PHONY: clean
clean:  ## clean all build and test artifacts
	# not sure if we should remove generated stuff
	# keep the output directory itself
	#-rm -rf output/*
	rm -rf bin
	@echo "removing test files that might be owned by root"
	sudo rm -rf /var/tmp/bib-tests

.PHONY: test
test:  ## run all tests - Be aware that the tests take a really long time
	cd bib && go test -race ./...
	@echo "Be aware that the tests take a really long time"
	@echo "Running tests as root"
	sudo -E pip install --user -r test/requirements.txt
	sudo -E pytest -s -v

.PHONY: build
build: build-binary  ## shortcut for build-binary

.PHONY: build-binary
build-binary:  ## build the binaries (multiple architectures)
	./build.sh

.PHONY: build-container
build-container:  ## build the bootc-image-builder container
	sudo podman build --tag bootc-image-builder .

.PHONY: push-check
push-check: build-binary build-container test  ## run all checks and tests before a pushing your code
	cd bib; go fmt ./...
	@if [ 0 -ne $$(git status --porcelain --untracked-files|wc -l) ]; then \
	    echo "There should be no changed or untracked files"; \
	    git status --porcelain --untracked-files; \
	    exit 1; \
	fi
	@echo "All looks good - congratulations"

$(GOLANGCI_LINT_BIN):
	@echo "golangci-lint does not seem to be installed"
	@read -p "Press <ENTER> to install it or <CTRL>-c to abort"
	$(GO_BINARY) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) || \
	( echo "if the go version is a problem, you can set GO_BINARY e.g. GO_BINARY=go.1.23.8 \
	      after installing it e.g. go install golang.org/dl/go1.23.8@latest" ; exit 1 )

.PHONY: lint
lint: $(GOLANGCI_LINT_BIN)  ## run the linters to check for bad code
	cd bib && $(GOLANGCI_LINT_BIN) run
