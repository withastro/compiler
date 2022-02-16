ASTRO_VERSION = $(shell cat version.txt)

GO_FLAGS += "-ldflags=-s -w"

# Avoid embedding the build path in the executable for more reproducible builds
GO_FLAGS += -trimpath

astro: cmd/astro/*.go internal/*/*.go go.mod
	CGO_ENABLED=0 go build $(GO_FLAGS) ./cmd/astro

astro-wasm: cmd/astro/*.go internal/*/*.go go.mod
	tinygo build -gc leaking -scheduler asyncify -no-debug -o ./lib/compiler/astro.wasm -target wasm ./cmd/astro-wasm/astro-wasm.go
	cp ./lib/compiler/astro.wasm ./lib/compiler/deno/astro.wasm

# alias to make astro-wasm
wasm:
	make astro-wasm

# useful for local debugging, retains go stacktrace for panics
debug: cmd/astro/*.go internal/*/*.go go.mod
	tinygo build -gc leaking -scheduler asyncify -o ./lib/compiler/astro.wasm -target wasm ./cmd/astro-wasm/astro-wasm.go
	cp ./lib/compiler/astro.wasm ./lib/compiler/deno/astro.wasm

publish-node: 
	make astro-wasm
	cd lib/compiler && npm run build

clean:
	git clean -dxf
