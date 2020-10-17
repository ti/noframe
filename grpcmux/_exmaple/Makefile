all: clean build
clean:
	 rm -rf ./pkg
build: SHELL:=/bin/bash
build:
	mkdir -p ./pkg/go ./pkg/openapi
	docker run --rm -v $(shell pwd)/pkg/go:/build/go -v $(shell pwd)/pkg/openapi:/build/openapi -v $(shell pwd):/build/proto nanxi/protoc:go
