//go:build js && wasm

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"syscall/js"

	"github.com/norunners/vert"
	astro "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/printer"
	"github.com/snowpackjs/astro/internal/transform"
	wasm_utils "github.com/snowpackjs/astro/internal_wasm/utils"
	"golang.org/x/net/html/atom"
)

var done chan bool

func main() {
	js.Global().Set("__astro_transform", Transform())
	// This ensures that the WASM doesn't exit early
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

	as := jsString(options.Get("as"))
	if as == "" {
		as = "document"
	}

	internalURL := jsString(options.Get("internalURL"))
	if internalURL == "" {
		internalURL = "astro/internal"
	}

	sourcemap := jsString(options.Get("sourcemap"))
	if sourcemap == "<boolean: true>" {
		sourcemap = "both"
	}

	site := jsString(options.Get("site"))
	if site == "" {
		site = "https://astro.build"
	}

	preprocessStyle := options.Get("preprocessStyle")

	return transform.TransformOptions{
		As:              as,
		Scope:           hash,
		Filename:        filename,
		InternalURL:     internalURL,
		SourceMap:       sourcemap,
		Site:            site,
		PreprocessStyle: preprocessStyle,
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

// This is spawned as a goroutine to preprocess style nodes using an async function passed from JS
func preprocessStyle(i int, style *astro.Node, transformOptions transform.TransformOptions, cb func()) {
	defer cb()
	attrs := wasm_utils.GetAttrs(style)
	data, _ := wasm_utils.Await(transformOptions.PreprocessStyle.(js.Value).Invoke(style.FirstChild.Data, attrs))
	str := jsString(data[0])
	if str == "" {
		return
	}
	style.FirstChild.Data = str
}

func Transform() interface{} {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		source := jsString(args[0])
		hash := astro.HashFromSource(source)
		transformOptions := makeTransformOptions(js.Value(args[1]), hash)

		handler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			resolve := args[0]

			var doc *astro.Node

			if transformOptions.As == "document" {
				docNode, _ := astro.Parse(strings.NewReader(source))
				doc = docNode
			} else if transformOptions.As == "fragment" {
				nodes, _ := astro.ParseFragment(strings.NewReader(source), &astro.Node{
					Type:     astro.ElementNode,
					Data:     atom.Body.String(),
					DataAtom: atom.Body,
				})
				doc = &astro.Node{
					Type: astro.DocumentNode,
				}
				for i := 0; i < len(nodes); i++ {
					n := nodes[i]
					doc.AppendChild(n)
				}
			}

			// Hoist styles and scripts to the top-level
			transform.ExtractScriptsAndStyles(doc)

			// Pre-process styles
			// Important! These goroutines need to be spawned from this file or they don't work
			var wg sync.WaitGroup
			if len(doc.Styles) > 0 {
				if transformOptions.PreprocessStyle.(js.Value).IsUndefined() != true {
					for i, style := range doc.Styles {
						wg.Add(1)
						i := i
						go preprocessStyle(i, style, transformOptions, wg.Done)
					}
				}
			}
			// Wait for all the style goroutines to finish
			wg.Wait()

			// Perform CSS and element scoping as needed
			transform.Transform(doc, transformOptions)

			result := printer.PrintToJS(source, doc, transformOptions)

			switch transformOptions.SourceMap {
			case "external":
				resolve.Invoke(createExternalSourceMap(source, result, transformOptions))
				return nil
			case "both":
				resolve.Invoke(createBothSourceMap(source, result, transformOptions))
				return nil
			case "inline":
				resolve.Invoke(createInlineSourceMap(source, result, transformOptions))
				return nil
			}

			resolve.Invoke(vert.ValueOf(TransformResult{
				Code: string(result.Output),
				Map:  "",
			}))

			return nil
		})
		defer handler.Release()

		// Create and return the Promise object
		promiseConstructor := js.Global().Get("Promise")
		return promiseConstructor.New(handler)
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
	inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	return vert.ValueOf(TransformResult{
		Code: string(result.Output) + "\n" + inlineSourcemap,
		Map:  "",
	})
}

func createBothSourceMap(source string, result printer.PrintResult, transformOptions transform.TransformOptions) interface{} {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	return vert.ValueOf(TransformResult{
		Code: string(result.Output) + "\n" + inlineSourcemap,
		Map:  sourcemapString,
	})
}
