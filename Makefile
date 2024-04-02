.PHONY: all
all: build-binary build-container

.PHONY: clean
clean:
	# not sure if we should remove generated stuff
	# keep the output directory itself
	#-rm -rf output/*
	rm -rf bin
	@echo "removing test files that might be owned by root"
	sudo rm -rf /var/tmp/bib-tests

.PHONY: test
test:
	@echo "Be aware that the tests take a really long time"
	@echo "Running tests as root"
	sudo -E pip install --user -r test/requirements.txt
	sudo -E pytest -s -v

.PHONY: build-binary
build-binary:
	./build.sh

.PHONY: build-container
build-container:
	sudo podman build --tag bootc-image-builder .

.PHONY: push-check
push-check: build-binary build-container test
	cd bib; go fmt ./...
	@if [ 0 -ne $$(git status --porcelain --untracked-files|wc -l) ]; then \
	    echo "There should be no changed or untracked files"; \
	    git status --porcelain --untracked-files; \
	    exit 1; \
	fi
	@echo "All looks good - congratulations"
