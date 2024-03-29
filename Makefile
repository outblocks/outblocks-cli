DEV_MAKEFILES ?= tools/dev/makefiles

include $(DEV_MAKEFILES)/changelog.mk
include $(DEV_MAKEFILES)/go.mk
include $(DEV_MAKEFILES)/kind.mk
include $(DEV_MAKEFILES)/kubectl.mk

STARTING_VERSION := v0.1.2

generate: ## Generate code
	go generate ./...
