BINARY=ai

OS=$(shell uname -s)
ARCH=$(shell uname -m)

USEARCH_VERSION=2.15.1

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

.PHONY: build install_usearch install clean

install: install_usearch build
	@echo "Installing ai..."
	go install
