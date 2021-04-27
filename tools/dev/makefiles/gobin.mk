ifndef _include_gobin_mk
_include_gobin_mk := 1

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

include $(SELF_DIR)shared.mk

GOBIN_VERSION := 0.0.14
GOBIN := $(DEV_BIN_PATH)/gobin_$(GOBIN_VERSION)

$(GOBIN):
	$(info $(_bullet) Installing <gobin>)
	@mkdir -p $(DEV_BIN_PATH)
	curl -sSfL https://github.com/myitcv/gobin/releases/download/v$(GOBIN_VERSION)/$(OS)-amd64 -o $(GOBIN)
	chmod u+x $(GOBIN)

endif
