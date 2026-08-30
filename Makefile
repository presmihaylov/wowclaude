.PHONY: build test test-addon install serve check

build:
	go build -o bin/wowclaude ./cmd/wowclaude

test:
	go test ./...

test-addon:
	luajit addon/tests/run.lua
	luajit -bl addon/WoWClaude/UI.lua > /dev/null
	@! grep -nE '#[a-zA-Z_(]|^local .* = \.\.\.|:match\(|string\.match|strtrim|[^a-zA-Z_.]print\(|SetSize|HookScript' addon/WoWClaude/*.lua | grep -v '^[^:]*:1:' || (echo "Lua 5.0 incompatibility above" && exit 1)

check: test test-addon
	go vet ./...

WOW_DIR ?= /Applications/World of Warcraft/_classic_era_

install: build
	./bin/wowclaude install --wow-dir "$(WOW_DIR)"

serve: build
	./bin/wowclaude serve --wow-dir "$(WOW_DIR)"
