.PHONY: all
all: build-binary build-container

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
