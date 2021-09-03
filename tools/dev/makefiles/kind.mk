ifndef _include_kind_mk
_include_kind_mk := 1
_kind_mk_path := $(dir $(lastword $(MAKEFILE_LIST)))

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

include $(SELF_DIR)shared.mk
include $(SELF_DIR)kubectl.mk

KIND_VERSION ?= 0.11.1
KIND := $(DEV_BIN_PATH)/kind_$(KIND_VERSION)
KIND_CLUSTER_NAME ?= local
KIND_K8S_VERSION ?= 1.19.12
KIND_HOST_PORT ?= 80

BOOTSTRAP_CONTEXT := kind-$(KIND_CLUSTER_NAME)

$(KIND):
	$(info $(_bullet) Installing <kind>)
	@mkdir -p $(DEV_BIN_PATH)
	curl -sSfL https://kind.sigs.k8s.io/dl/v$(KIND_VERSION)/kind-$(OS)-$(ARCH) -o $(KIND)
	chmod u+x $(KIND)

clean: clean-kind

bootstrap: bootstrap-kind ## Bootstrap kind cluster

.PHONY: clean-kind bootstrap-kind

clean-kind bootstrap-kind: export PATH := bin:$(PATH)
clean-kind bootstrap-kind: export CLUSTER_NAME := $(KIND_CLUSTER_NAME)
clean-kind bootstrap-kind: export K8S_VERSION := $(KIND_K8S_VERSION)
clean-kind bootstrap-kind: export HOST_PORT := $(KIND_HOST_PORT)

clean-kind: $(KIND) ## Delete kind cluster
	$(info $(_bullet) Cleaning <kind>)
	$(dir $(_kind_mk_path))scripts/clean-kind

bootstrap-kind: $(KUBECTL) $(KIND)
	$(info $(_bullet) Bootstraping <kind>)
	$(dir $(_kind_mk_path))scripts/bootstrap-kind

endif
