GONAMESPACE := github.com/cgwalters/osbuildbootc
PREFIX ?= /usr
DESTDIR ?=

.PHONY: all bin/osbuildbootc install clean vendor
all: bin/osbuildbootc

src:=$(shell find src -maxdepth 1 -type f -executable -print)
GOARCH:=$(shell uname -m)
ifeq ($(GOARCH),x86_64)
        GOARCH="amd64"
else ifeq ($(GOARCH),aarch64)
        GOARCH="arm64"
endif

bin/osbuildbootc:
	cd cmd && go build -mod vendor -o ../$@

check:
	(cd cmd && go test -mod=vendor)
	go test -mod=vendor $(GONAMESPACE)/internal/pkg/bashexec

clean:
	rm -rfv bin

install:
	install -d $(DESTDIR)$(PREFIX)/lib/osbuildbootc
	install -D -t $(DESTDIR)$(PREFIX)/lib/osbuildbootc $$(find src/ -maxdepth 1 -type f)
	install -D -t $(DESTDIR)$(PREFIX)/bin bin/osbuildbootc

vendor:
	@go mod vendor
	@go mod tidy

