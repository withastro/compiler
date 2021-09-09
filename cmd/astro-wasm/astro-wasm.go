// +build js,wasm
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/norunners/vert"
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
	filename := jsString(options.Get("sourcefile"))
	if filename == "" {
		filename = "<stdin>"
	}

	internalURL := jsString(options.Get("internalURL"))
	if internalURL == "" {
		internalURL = "astro/internal"
	}

	sourcemap := jsString(options.Get("sourcemap"))
	if sourcemap == "<boolean: true>" {
		sourcemap = "both"
	}

	return transform.TransformOptions{
		Scope:       hash,
		Filename:    filename,
		InternalURL: internalURL,
		SourceMap:   sourcemap,
	}
}

type RawSourceMap struct {
	File           string   `js:"file"`
	Mappings       string   `js:"mappings"`
	Names          []string `js:"names"`
	Sources        []string `js:"sources"`
	SourcesContent []string `js:"sourcesContent"`
	Version        int      `js:"version"`
}

type TransformResult struct {
	Code string `js:"code"`
	Map  string `js:"map"`
}

func Transform(this js.Value, args []js.Value) interface{} {
	source := jsString(args[0])

	doc, _ := astro.Parse(strings.NewReader(source))
	hash := astro.HashFromSource(source)
	transformOptions := makeTransformOptions(js.Value(args[1]), hash)

	transform.Transform(doc, transformOptions)

	result := printer.PrintToJS(source, doc, transformOptions)

	switch transformOptions.SourceMap {
	case "external":
		return createExternalSourceMap(source, result, transformOptions)
	case "both":
		return createBothSourceMap(source, result, transformOptions)
	case "inline":
		return createInlineSourceMap(source, result, transformOptions)
	}

	return vert.ValueOf(TransformResult{
		Code: string(result.Output),
		Map:  "",
	})
}

func createSourceMapString(source string, result printer.PrintResult, transformOptions transform.TransformOptions) string {
	sourcesContent, _ := json.Marshal(source)
	sourcemap := RawSourceMap{
		Version:        3,
		Sources:        []string{transformOptions.Filename},
		SourcesContent: []string{string(sourcesContent)},
		Mappings:       string(result.SourceMapChunk.Buffer),
	}
	return fmt.Sprintf(`{
  "version": 3,
  "sources": ["%s"],
  "sourcesContent": [%s],
  "mappings": "%s",
  "names": []
}`, sourcemap.Sources[0], sourcemap.SourcesContent[0], sourcemap.Mappings)
}

func createExternalSourceMap(source string, result printer.PrintResult, transformOptions transform.TransformOptions) interface{} {
	return vert.ValueOf(TransformResult{
		Code: string(result.Output),
		Map:  createSourceMapString(source, result, transformOptions),
	})
}

func createInlineSourceMap(source string, result printer.PrintResult, transformOptions transform.TransformOptions) interface{} {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//@ sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	return vert.ValueOf(TransformResult{
		Code: string(result.Output) + "\n" + inlineSourcemap,
		Map:  "",
	})
}

func createBothSourceMap(source string, result printer.PrintResult, transformOptions transform.TransformOptions) interface{} {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//@ sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	return vert.ValueOf(TransformResult{
		Code: string(result.Output) + "\n" + inlineSourcemap,
		Map:  sourcemapString,
	})
}
