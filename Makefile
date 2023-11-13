GONAMESPACE := github.com/cgwalters/osbuildbootc
PREFIX ?= /usr
DESTDIR ?=

BINARIES := osbuildbootc osbuild-deploy-container

.PHONY: all bin install clean vendor
all: bin

src:=$(shell find src -maxdepth 1 -type f -executable -print)
GOARCH:=$(shell uname -m)
ifeq ($(GOARCH),x86_64)
        GOARCH="amd64"
else ifeq ($(GOARCH),aarch64)
        GOARCH="arm64"
endif

bin:
	(cd cmd && go build -mod vendor -o ../bin/osbuildbootc)
	(top=$$(pwd); cd cmd/osbuild-deploy-container && go build -mod vendor -o $${top}/bin/osbuild-deploy-container)

check:
	(cd cmd && go test -mod=vendor)
	go test -mod=vendor $(GONAMESPACE)/internal/pkg/bashexec

clean:
	rm -rfv bin

install:
	install -d $(DESTDIR)$(PREFIX)/lib/osbuildbootc
	install -D -t $(DESTDIR)$(PREFIX)/lib/osbuildbootc $$(find src/ -maxdepth 1 -type f)
	install -D -t $(DESTDIR)$(PREFIX)/bin $(addprefix bin/,$(BINARIES))

vendor:
	@go mod vendor
	@go mod tidy

