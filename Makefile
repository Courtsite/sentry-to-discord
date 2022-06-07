.DEFAULT_GOAL := help

export UID := $(shell id -u $(USER))
export GID := $(shell id -g $(USER))

fmt: # gofmt project
	gofmt -s -w .
.PHONY: fmt

lint: # Run linters; usage: make lint [pkg=./path/to/...]
	golangci-lint run $(pkg)
.PHONY: lint

clean: # Remove binaries, logs, temp files
	rm -f build/*
.PHONY: clean

build: # Build docker image for current revision
	GOOS=linux GOARCH=amd64 go build -v -ldflags '-d -s -w' -a -tags netgo -installsuffix netgo -o build/sentry-to-discord .
.PHONY: build

update: # Update lambda with new binary
	AWS_PROFILE=srp aws lambda update-function-code \
		--function-name sentry-to-discord \
		--zip-file fileb://./build/sentry-to-discord.zip \
		--publish
.PHONY: update

pack:
	zip build/sentry-to-discord.zip build/sentry-to-discord
.PHONY: pack

release: # Build and push to ECR an image for the current revision
	$(MAKE) clean
	$(MAKE) build
	$(MAKE) pack
	$(MAKE) update
.PHONY: release

help: # Print this help (default target)
	@echo "Available build targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?# .*$$' $(MAKEFILE_LIST) \
	| sed -n 's/^\(.*\): \(.*\)#\(.*\)/make \1;;#\3/p' \
	| column -t -s ';;'
.PHONY: help
