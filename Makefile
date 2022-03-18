GO_FLAGS += "-ldflags=-s -w"

# Avoid embedding the build path in the executable for more reproducible builds
GO_FLAGS += -trimpath


wasm: cmd/astro/*.go internal/*/*.go go.mod
	CGO_ENABLED=0 GOOS=js GOARCH=wasm go build $(GO_FLAGS) -o ./packages/compiler/astro.wasm ./cmd/astro-wasm/astro-wasm.go
	cp ./packages/compiler/astro.wasm ./packages/compiler/deno/astro.wasm


publish-node: 
	make wasm
	cd packages/compiler && npm run build

clean:
	git clean -dxf
