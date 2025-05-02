GO_FLAGS += "-ldflags=-s -w"

# Uncomment for debugging
# compiler optimizations
# GO_FLAGS += "-gcflags=-m"

# Avoid embedding the build path in the executable for more reproducible builds
GO_FLAGS += -trimpath


wasm: internal/*/*.go go.mod
	CGO_ENABLED=0 GOOS=js GOARCH=wasm go build $(GO_FLAGS) -o ./packages/compiler/wasm/astro.wasm ./cmd/astro-wasm/astro-wasm.go


publish-node:
	make wasm
	cd packages/compiler && pnpm run build

clean:
	git clean -dxf
