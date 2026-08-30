.PHONY: build test test-addon install serve check

build:
	go build -o bin/wowclaude ./cmd/wowclaude

test:
	go test ./...

test-addon:
	luajit addon/tests/run.lua

check: test test-addon
	go vet ./...

WOW_DIR ?= /Applications/World of Warcraft/_classic_era_

install: build
	./bin/wowclaude install --wow-dir "$(WOW_DIR)"

serve: build
	./bin/wowclaude serve --wow-dir "$(WOW_DIR)"
