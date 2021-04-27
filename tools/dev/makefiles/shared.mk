ifndef _include_shared_mk
_include_shared_mk := 1

TOOLS_LATEST ?= https://github.com/23doors/dev-tools/archive/master.zip
OS ?= $(shell uname -s | tr [:upper:] [:lower:])
DEV_BIN_PATH ?= bin

.PHONY: help clean deps vendor generate format lint test test-coverage integration-test build bootrap deploy run dev debug

$(VERBOSE).SILENT:
.DEFAULT_GOAL := help

help: ## Help
	@cat $(sort $(MAKEFILE_LIST)) | grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' | sort | uniq

clean: clean-bin ## Clean targets

.PHONY: clean-bin git-dirty git-hooks

clean-bin: ## Clean dev tools binaries
	$(info $(_bullet) Cleaning <bin>)
	rm -rf $(DEV_BIN_PATH)

update-tools-existing: ## Update only existing scripts from dev tools
	$(info $(_bullet) Updating only existing dev tools to latest)
	curl -Ls $(TOOLS_LATEST) -o tmp.zip >/dev/null
	rm -rf _tmp
	unzip tmp.zip -d _tmp >/dev/null
	find tools/dev -type f | xargs -I '{}' mv _tmp/dev-tools-master/{} {}
	rm -rf tmp.zip _tmp

update-tools: ## Update fully dev tools
	$(info $(_bullet) Updating fully dev tools to latest)
	curl -Ls $(TOOLS_LATEST) -o tmp.zip >/dev/null
	rm -rf _tmp
	unzip tmp.zip -d _tmp >/dev/null
	mv _tmp/dev-tools-master/tools/dev tools/dev
	rm -rf tmp.zip _tmp

_bullet := $(shell printf "\033[34;1m▶\033[0m")

endif
