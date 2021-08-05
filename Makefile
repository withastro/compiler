ASTRO_VERSION = $(shell cat version.txt)

# Strip debug info
GO_FLAGS += "-ldflags=-s -w"

# Avoid embedding the build path in the executable for more reproducible builds
GO_FLAGS += -trimpath

astro: cmd/astro/*.go pkg/*/*.go internal/*/*.go go.mod
	CGO_ENABLED=0 go build $(GO_FLAGS) ./cmd/astro

publish-node: 
	cd lib/compiler && npm run build

clean:
	git clean -dxf
