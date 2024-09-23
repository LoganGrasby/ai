BINARY=ai

OS=$(shell uname -s)
ARCH=$(shell uname -m)

USEARCH_VERSION=2.15.1

SHELL := /bin/bash

CURRENT_SHELL := $(shell basename $$SHELL)

ifeq ($(CURRENT_SHELL),bash)
    PROFILE_FILE := $(HOME)/.bashrc
else ifeq ($(CURRENT_SHELL),zsh)
    PROFILE_FILE := $(HOME)/.zshrc
else
    PROFILE_FILE := $(HOME)/.profile
endif

# The following exports are added to the current shell profile if they are not already set
EXPORTS := \
    'export PATH=$$PATH:/usr/local/go/bin' \
    'export GOPATH=$$HOME/go' \
    'export PATH=$$PATH:$$GOPATH/bin' \
    'export CGO_CFLAGS="-I/usr/local/include"' \
    'export CGO_LDFLAGS="-L/usr/local/lib"' \
    'export DYLD_LIBRARY_PATH=/usr/local/lib:$$DYLD_LIBRARY_PATH'

all: update_profile

update_profile:
	@echo "Current shell: $(CURRENT_SHELL)"
	@echo "Profile to update: $(PROFILE_FILE)"
	@echo "Enter your password to continue install..."
	@sudo -v
	@updated=0; \
	if [ -f $(PROFILE_FILE) ]; then \
		echo "Processing $(PROFILE_FILE)..."; \
	else \
		echo "Unable to locate shell profile..."; \
		exit 1; \
	fi; \
	for line in $(EXPORTS); do \
		if ! sudo grep -Fq "$$line" $(PROFILE_FILE); then \
			echo "$$line" | sudo tee -a $(PROFILE_FILE) > /dev/null; \
			echo "Added: $$line"; \
			updated=1; \
		fi; \
	done; \
	if [ $$updated -eq 1 ]; then \
		echo "Environment variables updated in $(PROFILE_FILE)."; \
	else \
		echo "All environment variables are already set in $(PROFILE_FILE)."; \
	fi

notify_install:
	@if [ -f $(PROFILE_FILE) ]; then \
		if sudo grep -q "export PATH=\$$PATH:/usr/local/go/bin" $(PROFILE_FILE); then \
			echo "Profile has been updated. Please run 'source $(PROFILE_FILE)' to apply the changes."; \
		fi; \
	fi


ifeq ($(OS),Linux)
    ifeq ($(ARCH),x86_64)
        USEARCH_ARCH=amd64
        GO_BUILD_TAGS=linux,amd64
    else ifeq ($(ARCH),aarch64)
        USEARCH_ARCH=arm64
        GO_BUILD_TAGS=linux,arm64
    endif
    USEARCH_PACKAGE=usearch_linux_$(USEARCH_ARCH)_$(USEARCH_VERSION).deb
    USEARCH_URL=https://github.com/unum-cloud/usearch/releases/download/v$(USEARCH_VERSION)/$(USEARCH_PACKAGE)
else ifeq ($(OS),Darwin)
    ifeq ($(ARCH),x86_64)
        USEARCH_ARCH=x86_64
        GO_BUILD_TAGS=darwin,amd64
    else ifeq ($(ARCH),arm64)
        USEARCH_ARCH=arm64
        GO_BUILD_TAGS=darwin,arm64
    endif
    USEARCH_PACKAGE=usearch_macos_$(USEARCH_ARCH)_$(USEARCH_VERSION).zip
    USEARCH_URL=https://github.com/unum-cloud/usearch/releases/download/v$(USEARCH_VERSION)/$(USEARCH_PACKAGE)
else ifeq ($(OS),Windows_NT)
    USEARCH_ARCH=$(ARCH)
    GO_BUILD_TAGS=windows,$(ARCH)
endif

install_usearch:
ifeq ($(OS),Linux)
	@echo "Installing USearch on Linux..."
	wget -q $(USEARCH_URL) -O $(USEARCH_PACKAGE)
	sudo dpkg -i $(USEARCH_PACKAGE)
	rm $(USEARCH_PACKAGE)
	sudo ldconfig
else ifeq ($(OS),Darwin)
	@echo "Installing USearch on macOS..."
	wget -q $(USEARCH_URL) -O $(USEARCH_PACKAGE)
	unzip $(USEARCH_PACKAGE)
	sudo mv build_release/libusearch_c.dylib /usr/local/lib/
	sudo mv c/usearch.h /usr/local/include/
	rm -r build_release c
	rm $(USEARCH_PACKAGE)
else ifeq ($(OS),Windows_NT)
	@echo "Installing USearch on Windows..."
	powershell -Command "& {iwr -outf winlibinstaller.bat https://raw.githubusercontent.com/unum-cloud/usearch/main/winlibinstaller.bat; .\winlibinstaller.bat}"
endif

build: install_usearch
	@echo "Building ai..."
	go build -tags "$(GO_BUILD_TAGS)" -o ${BINARY} .

clean:
	@echo "Cleaning up..."
	go clean
	rm -f ${BINARY}

go_install:
	@echo "Installing ai..."
	go install

install: update_profile install_usearch build go_install
	@if [ -f .profile_status ] && [ "$$(cat .profile_status | cut -d'=' -f2)" = "1" ]; then \
		$(MAKE) notify_install; \
	fi
	@rm -f .profile_status
	@echo "ai installed successfully!"

.PHONY: build install_usearch install clean notify_install
