ifndef _include_kubectl_mk
_include_kubectl_mk := 1

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

include $(SELF_DIR)shared.mk

KUBECTL_VERSION ?= 1.17.15
KUBECTL := $(DEV_BIN_PATH)/kubectl_$(KUBECTL_VERSION)

$(KUBECTL):
	$(info $(_bullet) Installing <kubectl>)
	@mkdir -p $(DEV_BIN_PATH)
	curl -sSfL https://storage.googleapis.com/kubernetes-release/release/v$(KUBECTL_VERSION)/bin/$(OS)/amd64/kubectl -o $(KUBECTL)
	chmod u+x $(KUBECTL)

endif

