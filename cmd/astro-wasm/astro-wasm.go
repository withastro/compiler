//go:build js && wasm
// +build js,wasm

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"syscall/js"

	"github.com/norunners/vert"
	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/printer"
	t "github.com/withastro/compiler/internal/t"
	"github.com/withastro/compiler/internal/transform"
	wasm_utils "github.com/withastro/compiler/internal_wasm/utils"
)

var done chan bool

func main() {
	js.Global().Set("@astrojs/compiler", js.ValueOf(make(map[string]interface{})))
	module := js.Global().Get("@astrojs/compiler")
	module.Set("transform", Transform())
	module.Set("parse", Parse())
	module.Set("convertToTSX", ConvertToTSX())

	<-make(chan struct{})
}

func jsString(j js.Value) string {
	if j.Equal(js.Undefined()) || j.Equal(js.Null()) {
		return ""
	}
	return j.String()
}

func jsBool(j js.Value) bool {
	if j.Equal(js.Undefined()) || j.Equal(js.Null()) {
		return false
	}
	return j.Bool()
}

func makeParseOptions(options js.Value) t.ParseOptions {
	position := true

	pos := options.Get("position")
	if !pos.IsNull() && !pos.IsUndefined() {
		position = pos.Bool()
	}

	return t.ParseOptions{
		Position: position,
	}
}

func makeTransformOptions(options js.Value, hash string) transform.TransformOptions {
	filename := jsString(options.Get("sourcefile"))
	if filename == "" {
		filename = "<stdin>"
	}

	pathname := jsString(options.Get("pathname"))
	if pathname == "" {
		pathname = "<stdin>"
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

	projectRoot := jsString(options.Get("projectRoot"))
	if projectRoot == "" {
		projectRoot = "."
	}

	compact := false
	if jsBool(options.Get("compact")) {
		compact = true
	}

	staticExtraction := false
	if jsBool(options.Get("experimentalStaticExtraction")) {
		staticExtraction = true
	}

	preprocessStyle := options.Get("preprocessStyle")

	return transform.TransformOptions{
		Scope:            hash,
		Filename:         filename,
		Pathname:         pathname,
		InternalURL:      internalURL,
		SourceMap:        sourcemap,
		Site:             site,
		ProjectRoot:      projectRoot,
		Compact:          compact,
		PreprocessStyle:  preprocessStyle,
		StaticExtraction: staticExtraction,
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

type HoistedScript struct {
	Code string `js:"code"`
	Src  string `js:"src"`
	Type string `js:"type"`
}

type HydratedComponent struct {
	ExportName   string `js:"exportName"`
	Specifier    string `js:"specifier"`
	ResolvedPath string `js:"resolvedPath"`
}

type ParseResult struct {
	AST string `js:"ast"`
}

type TSXResult struct {
	Code string `js:"code"`
	Map  string `js:"map"`
}

type TransformResult struct {
	Code                 string              `js:"code"`
	Map                  string              `js:"map"`
	CSS                  []string            `js:"css"`
	Scripts              []HoistedScript     `js:"scripts"`
	HydratedComponents   []HydratedComponent `js:"hydratedComponents"`
	ClientOnlyComponents []HydratedComponent `js:"clientOnlyComponents"`
}

// This is spawned as a goroutine to preprocess style nodes using an async function passed from JS
func preprocessStyle(i int, style *astro.Node, transformOptions transform.TransformOptions, cb func()) {
	defer cb()
	if style.FirstChild == nil {
		return
	}
	attrs := wasm_utils.GetAttrs(style)
	data, _ := wasm_utils.Await(transformOptions.PreprocessStyle.(js.Value).Invoke(style.FirstChild.Data, attrs))
	// note: Rollup (and by extension our Astro Vite plugin) allows for "undefined" and "null" responses if a transform wishes to skip this occurrence
	if data[0].Equal(js.Undefined()) || data[0].Equal(js.Null()) {
		return
	}
	str := jsString(data[0].Get("code"))
	if str == "" {
		return
	}
	style.FirstChild.Data = str
}

func Parse() interface{} {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		source := jsString(args[0])
		parseOptions := makeParseOptions(js.Value(args[1]))

		var doc *astro.Node
		doc, err := astro.Parse(strings.NewReader(source))
		if err != nil {
			fmt.Println(err)
		}
		result := printer.PrintToJSON(source, doc, parseOptions)

		return vert.ValueOf(ParseResult{
			AST: string(result.Output),
		})
	})
}

func ConvertToTSX() interface{} {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		source := jsString(args[0])
		transformOptions := makeTransformOptions(js.Value(args[1]), "XXXXXX")

		var doc *astro.Node
		doc, err := astro.Parse(strings.NewReader(source))
		if err != nil {
			fmt.Println(err)
		}
		result := printer.PrintToTSX(source, doc, transformOptions)

		return vert.ValueOf(TSXResult{
			Code: string(result.Output),
			Map:  createSourceMapString(source, result, transformOptions),
		})
	})
}

func Transform() interface{} {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		source := jsString(args[0])
		hash := astro.HashFromSource(source)
		transformOptions := makeTransformOptions(js.Value(args[1]), hash)

		handler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			resolve := args[0]

			go func() {
				var doc *astro.Node

				doc, err := astro.Parse(strings.NewReader(source))
				if err != nil {
					fmt.Println(err)
				}

				// Hoist styles and scripts to the top-level
				transform.ExtractStyles(doc)

				// Pre-process styles
				// Important! These goroutines need to be spawned from this file or they don't work
				var wg sync.WaitGroup
				if len(doc.Styles) > 0 {
					if transformOptions.PreprocessStyle.(js.Value).Type() == js.TypeFunction {
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

				css := []string{}
				scripts := []HoistedScript{}
				hydratedComponents := []HydratedComponent{}
				clientOnlyComponents := []HydratedComponent{}
				// Only perform static CSS extraction if the flag is passed in.
				if transformOptions.StaticExtraction {
					css_result := printer.PrintCSS(source, doc, transformOptions)
					for _, bytes := range css_result.Output {
						css = append(css, string(bytes))
					}

					// Append hoisted scripts
					for _, node := range doc.Scripts {
						src := astro.GetAttribute(node, "src")
						script := HoistedScript{
							Src:  "",
							Code: "",
							Type: "",
						}
						if src != nil {
							script.Type = "external"
							script.Src = src.Val
						} else if node.FirstChild != nil {
							script.Type = "inline"
							script.Code = node.FirstChild.Data
						}
						scripts = append(scripts, script)
					}

					for _, c := range doc.HydratedComponents {
						hydratedComponents = append(hydratedComponents, HydratedComponent{
							ExportName:   c.ExportName,
							Specifier:    c.Specifier,
							ResolvedPath: c.ResolvedPath,
						})
					}

					for _, c := range doc.ClientOnlyComponents {
						clientOnlyComponents = append(clientOnlyComponents, HydratedComponent{
							ExportName:   c.ExportName,
							Specifier:    c.Specifier,
							ResolvedPath: c.ResolvedPath,
						})
					}
				}

				result := printer.PrintToJS(source, doc, len(css), transformOptions)

				var value interface{}
				switch transformOptions.SourceMap {
				case "external":
					value = createExternalSourceMap(source, result, css, &scripts, &hydratedComponents, &clientOnlyComponents, transformOptions)
				case "both":
					value = createBothSourceMap(source, result, css, &scripts, &hydratedComponents, &clientOnlyComponents, transformOptions)
				case "inline":
					value = createInlineSourceMap(source, result, css, &scripts, &hydratedComponents, &clientOnlyComponents, transformOptions)
				default:
					value = vert.ValueOf(TransformResult{
						CSS:                  css,
						Code:                 string(result.Output),
						Map:                  "",
						Scripts:              scripts,
						HydratedComponents:   hydratedComponents,
						ClientOnlyComponents: clientOnlyComponents,
					})
				}

				resolve.Invoke(value)
			}()

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

func createExternalSourceMap(source string, result printer.PrintResult, css []string, scripts *[]HoistedScript, hydratedComponents *[]HydratedComponent, clientOnlyComponents *[]HydratedComponent, transformOptions transform.TransformOptions) interface{} {
	return vert.ValueOf(TransformResult{
		CSS:                  css,
		Code:                 string(result.Output),
		Map:                  createSourceMapString(source, result, transformOptions),
		Scripts:              *scripts,
		HydratedComponents:   *hydratedComponents,
		ClientOnlyComponents: *clientOnlyComponents,
	})
}

func createInlineSourceMap(source string, result printer.PrintResult, css []string, scripts *[]HoistedScript, hydratedComponents *[]HydratedComponent, clientOnlyComponents *[]HydratedComponent, transformOptions transform.TransformOptions) interface{} {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	return vert.ValueOf(TransformResult{
		CSS:                  css,
		Code:                 string(result.Output) + "\n" + inlineSourcemap,
		Map:                  "",
		Scripts:              *scripts,
		HydratedComponents:   *hydratedComponents,
		ClientOnlyComponents: *clientOnlyComponents,
	})
}

func createBothSourceMap(source string, result printer.PrintResult, css []string, scripts *[]HoistedScript, hydratedComponents *[]HydratedComponent, clientOnlyComponents *[]HydratedComponent, transformOptions transform.TransformOptions) interface{} {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	return vert.ValueOf(TransformResult{
		CSS:                  css,
		Code:                 string(result.Output) + "\n" + inlineSourcemap,
		Map:                  sourcemapString,
		Scripts:              *scripts,
		HydratedComponents:   *hydratedComponents,
		ClientOnlyComponents: *clientOnlyComponents,
	})
}
