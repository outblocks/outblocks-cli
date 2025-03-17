ifndef _include_changelog_mk
_include_changelog_mk = 1

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

include $(SELF_DIR)shared.mk

STARTING_VERSION := v0.1.0
GITCHGLOG_VERSION ?= 0.15.4
GITCHGLOG := $(DEV_BIN_PATH)/git-chglog_$(GITCHGLOG_VERSION)
SEMVERBOT_VERSION ?= 1.7.2
SEMVERBOT := $(DEV_BIN_PATH)/sbot_$(SEMVERBOT_VERSION)

SEMVER_PREDICT_VERSION = $(shell $(SEMVERBOT) predict version)
VERSION ?= $(SEMVER_PREDICT_VERSION)

.PHONY: release

$(GITCHGLOG):
	$(info $(_bullet) Installing <git-chglog>")
	@mkdir -p $(DEV_BIN_PATH)
	curl -sSfL https://github.com/git-chglog/git-chglog/releases/download/v$(GITCHGLOG_VERSION)/git-chglog_$(GITCHGLOG_VERSION)_$(OS)_$(ARCH).tar.gz | tar -C $(DEV_BIN_PATH) -xz git-chglog
	mv $(DEV_BIN_PATH)/git-chglog $(GITCHGLOG)
	chmod u+x $(GITCHGLOG)

$(SEMVERBOT):
	$(info $(_bullet) Installing <semverbot>")
	@mkdir -p $(DEV_BIN_PATH)
	curl -sSfL https://github.com/restechnica/semverbot/releases/download/v$(SEMVERBOT_VERSION)/sbot-$(OS)-$(ARCH) -o $(SEMVERBOT)
	chmod u+x $(SEMVERBOT)

release: $(GITCHGLOG) $(SEMVERBOT)

release: ## Create new release and update changelog
	$(GITCHGLOG) --next-tag v$(VERSION) v$(VERSION)..

	echo
	echo "Releasing v$(VERSION). Continue? [y/N] " && read ans && [ $${ans:-N} = y ]

	$(GITCHGLOG) --next-tag v$(VERSION) --output CHANGELOG.md $(STARTING_VERSION)..
	git add CHANGELOG.md
	git commit -m "changelog update"
	git push

	git tag -a v$(VERSION) -m $(VERSION)
	git push origin v$(VERSION)

endif
