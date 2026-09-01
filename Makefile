GO ?= go
GO_PKG ?= .
OS_LIST ?= linux darwin windows
ARCH_LIST ?= amd64 arm64
BUILD_VARIANTS ?= debug release

OUTPUT_DIR ?= build
APP_NAME ?= kakaobot

GO_DEBUG_FLAGS ?= -trimpath -gcflags="all=-N -l"
GO_RELEASE_FLAGS ?= -trimpath -ldflags="-s -w -extldflags '-static'"

# collect all go files recursively for dependency tracking
rwildcard = $(wildcard $(1)/$(2)) $(foreach d,$(wildcard $(1)/*),$(call rwildcard,$d,$(2)))
GO_FILES := $(call rwildcard,.,*.go)

artifact_dir = $(OUTPUT_DIR)/$(1)-$(2)/$(3)
artifact = $(call artifact_dir,$(1),$(2),$(3))/$(APP_NAME)$(if $(filter windows,$(1)),.exe,)

define build_target
$(call artifact,$(1),$(2),$(3)): $(GO_FILES) | $(call artifact_dir,$(1),$(2),$(3))
	GOOS=$(1) GOARCH=$(2) $(if $(filter release,$(3)),CGO_ENABLED=0) \
	$(GO) build \
	$(if $(filter release,$(3)),$(GO_RELEASE_FLAGS),$(GO_DEBUG_FLAGS)) \
	-o $$@ $(GO_PKG)
endef

define token1
$(word 1,$(subst -, ,$(1)))
endef

define token2
$(word 2,$(subst -, ,$(1)))
endef

define token3
$(word 3,$(subst -, ,$(1)))
endef

OS_ARCH_PAIRS := $(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(os)-$(arch)))
OS_VARIANT_PAIRS := $(foreach os,$(OS_LIST),$(foreach var,$(BUILD_VARIANTS),$(os)-$(var)))
ARCH_VARIANT_PAIRS := $(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(arch)-$(var)))
FULL_SELECTOR_KEYS := $(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(os)-$(arch)-$(var))))

FULL_TARGETS := $(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(call artifact,$(os),$(arch),$(var)))))

define all_for_os
$(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(call artifact,$(1),$(arch),$(var))))
endef

define all_for_arch
$(foreach os,$(OS_LIST),$(foreach var,$(BUILD_VARIANTS),$(call artifact,$(os),$(1),$(var))))
endef

define all_for_variant
$(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(call artifact,$(os),$(arch),$(1))))
endef

define all_for_os_arch
$(foreach var,$(BUILD_VARIANTS),$(call artifact,$(1),$(2),$(var)))
endef

define all_for_os_variant
$(foreach arch,$(ARCH_LIST),$(call artifact,$(1),$(arch),$(2)))
endef

define all_for_arch_variant
$(foreach os,$(OS_LIST),$(call artifact,$(os),$(1),$(2)))
endef

.PHONY: all test lint clean $(OS_LIST) $(ARCH_LIST) $(BUILD_VARIANTS) $(OS_ARCH_PAIRS) $(OS_VARIANT_PAIRS) $(ARCH_VARIANT_PAIRS) $(FULL_SELECTOR_KEYS)

all: $(FULL_TARGETS)

test:
	$(GO) test -v -race ./...

lint:
	$(GO) vet ./...

clean:
	@rm -rf $(OUTPUT_DIR)

$(OUTPUT_DIR):
	@mkdir -p $(OUTPUT_DIR)

$(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(call artifact_dir,$(os),$(arch),$(var))))):
	@mkdir -p $@

$(OS_LIST):
	@$(MAKE) $(call all_for_os,$(@F))

$(ARCH_LIST):
	@$(MAKE) $(call all_for_arch,$(@F))

$(BUILD_VARIANTS):
	@$(MAKE) $(call all_for_variant,$(@F))

$(OS_ARCH_PAIRS):
	@$(MAKE) $(call all_for_os_arch,$(call token1,$(@F)),$(call token2,$(@F)))

$(OS_VARIANT_PAIRS):
	@$(MAKE) $(call all_for_os_variant,$(call token1,$(@F)),$(call token2,$(@F)))

$(ARCH_VARIANT_PAIRS):
	@$(MAKE) $(call all_for_arch_variant,$(call token1,$(@F)),$(call token2,$(@F)))

$(FULL_SELECTOR_KEYS):
	@$(MAKE) $(call artifact,$(call token1,$(@F)),$(call token2,$(@F)),$(call token3,$(@F)))

$(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(eval $(call build_target,$(os),$(arch),$(var))))))

help:
	@echo "make all: build all targets"
	@echo "make clean: remove $(OUTPUT_DIR)"
	@echo "make <os>|<arch>|<variant>|<os>-<arch>|<os>-<variant>|<arch>-<variant>|<os>-<arch>-<variant>"
	@echo "artifacts: $(OUTPUT_DIR)/<os>-<arch>/<variant>/$(APP_NAME)[.exe]"

%:
	@echo "Unknown target '$@'"
	@echo "Run 'make help' for valid patterns"
	@exit 1
