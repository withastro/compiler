package main

import (
	"encoding/json"
	"strings"
	"syscall/js"

	astro "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/printer"
	"github.com/snowpackjs/astro/internal/transform"
)

func main() {
	js.Global().Set("__astro_transform", js.FuncOf(Transform))
	<-make(chan bool)
}

func jsString(j js.Value) string {
	if j.IsUndefined() || j.IsNull() {
		return ""
	}
	return j.String()
}

func makeTransformOptions(options js.Value, hash string) transform.TransformOptions {
	filename := jsString(options.Get("filename"))
	if filename == "" {
		filename = "file.astro"
	}

	internalURL := jsString(options.Get("internalURL"))
	if internalURL == "" {
		internalURL = "astro/internal"
	}

	return transform.TransformOptions{
		Scope:       hash,
		Filename:    filename,
		InternalURL: internalURL,
	}
}

type TransformResult struct {
	Code string       `json:"code"`
	Map  RawSourceMap `json:"map"`
}

type RawSourceMap struct {
	file           string   `json:"file"`
	mappings       string   `json:"mappings"`
	names          []string `json:"names"`
	sources        []string `json:"sources"`
	sourcesContent []string `json:"sourcesContent"`
	version        int      `json:"version"`
}

func Transform(this js.Value, args []js.Value) interface{} {
	source := jsString(args[0])

	doc, _ := astro.Parse(strings.NewReader(source))
	hash := astro.HashFromSource(source)
	transformOptions := makeTransformOptions(js.Value(args[1]), hash)

	transform.Transform(doc, transformOptions)

	result := printer.PrintToJS(source, doc, transformOptions)
	sourcesContent, _ := json.Marshal(source)

	code := result.Output

	return TransformResult{
		Code: string(code),
		Map: RawSourceMap{
			file:           transformOptions.Filename,
			mappings:       string(result.SourceMapChunk.Buffer),
			names:          make([]string, 0),
			sources:        []string{transformOptions.Filename},
			sourcesContent: []string{string(sourcesContent)},
			version:        3,
		},
	}
}
