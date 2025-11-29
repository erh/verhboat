
GO_BUILD_ENV :=
GO_BUILD_FLAGS :=
MODULE_BINARY := bin/verhboat

ifeq ($(VIAM_TARGET_OS), windows)
	GO_BUILD_ENV += GOOS=windows GOARCH=amd64
	GO_BUILD_FLAGS := -tags no_cgo
	MODULE_BINARY = bin/verhboat.exe
endif

$(MODULE_BINARY): Makefile go.mod *.go cmd/module/*.go web-cam/dist/index.html
	$(GO_BUILD_ENV) go build $(GO_BUILD_FLAGS) -o $(MODULE_BINARY) cmd/module/main.go

lint:
	gofmt -s -w .

update:
	go get go.viam.com/rdk@latest
	go mod tidy

test:
	go test ./...

module.tar.gz: meta.json $(MODULE_BINARY)
ifneq ($(VIAM_TARGET_OS), windows)
	strip $(MODULE_BINARY)
endif
	tar czf $@ meta.json $(MODULE_BINARY)

module: test module.tar.gz

all: test module.tar.gz

setup:
	go mod tidy
	which npm > /dev/null 2>&1 || apt -y install nodejs

web-cam/dist/index.html: web-cam/*.json web-cam/src/*.svelte
	cd web-cam && npm install && NODE_ENV=development npm run build
