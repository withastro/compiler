package printer

import (
	"github.com/snowpackjs/astro/internal/sourcemap"
)

type PrintResult struct {
	Output         []byte
	SourceMapChunk sourcemap.Chunk
}

type printer struct {
	output  []byte
	builder sourcemap.ChunkBuilder
}
