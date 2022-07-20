//go:build js && wasm
// +build js,wasm

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"syscall/js"
	"unicode"

	"github.com/norunners/vert"
	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/js_scanner"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/printer"
	"github.com/withastro/compiler/internal/sourcemap"
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
		site = ""
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
	Map  string `js:"map"`
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
						defineVars := astro.GetAttribute(node, "define:vars")
						src := astro.GetAttribute(node, "src")
						script := HoistedScript{
							Src:  "",
							Code: "",
							Type: "",
							Map:  "",
						}

						if src != nil {
							script.Type = "external"
							script.Src = src.Val
						} else if node.FirstChild != nil && defineVars != nil {
							script.Type = "define:vars"
							keys := js_scanner.GetObjectKeys([]byte(defineVars.Val))
							params := make([]byte, 0)
							for i, key := range keys {
								params = append(params, key...)
								if i < len(keys)-1 {
									params = append(params, ',')
								}
							}
							if transformOptions.SourceMap != "" {
								output := make([]byte, 0)
								builder := sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(source, len(strings.Split(source, "\n"))))
								sourcesContent, _ := json.Marshal(source)
								src := []byte(node.FirstChild.Data)
								hoisted := js_scanner.HoistImports(src)
								if len(node.FirstChild.Loc) > 0 {
									i := node.FirstChild.Loc[0].Start
									for _, statement := range hoisted.Hoisted {
										j := bytes.Index(src, statement)
										start := i + j
										for k, b := range statement {
											if k == 0 || !unicode.IsSpace(rune(b)) {
												builder.AddSourceMapping(loc.Loc{Start: start}, output)
											}
											output = append(output, b)
											start += 1
										}
										builder.AddSourceMapping(loc.Loc{}, output)
										output = append(output, '\n')
									}
									builder.AddSourceMapping(loc.Loc{}, output)
									output = append(output, []byte(fmt.Sprintf(`import deserialize from 'astro/client/deserialize.js'; export default (str) => (async function({%s}) {%s`, params, "\n"))...)
									body := bytes.TrimSpace(hoisted.Body)
									j := bytes.Index(src, body)
									start := i + j
									for _, ln := range bytes.Split(body, []byte{'\n'}) {
										content := []byte(ln)
										content = append(content, '\n')
										for _, b := range content {
											if !unicode.IsSpace(rune(b)) {
												builder.AddSourceMapping(loc.Loc{Start: start}, output)
											}
											output = append(output, b)
											start += 1
										}
									}
									builder.AddSourceMapping(loc.Loc{}, output)
									output = append(output, []byte(fmt.Sprintf(`%s})(deserialize(str))`, "\n"))...)
									output = append(output, '\n')
								} else {
									for _, statement := range hoisted.Hoisted {
										output = append(output, statement...)
									}
									output = append(output, []byte(fmt.Sprintf(`import deserialize from 'astro/client/deserialize.js'; export default (str) => (async function({%s}) {%s`, params, "\n"))...)
									output = append(output, hoisted.Body...)
									output = append(output, []byte(fmt.Sprintf(`%s})(deserialize(str))`, "\n"))...)
								}

								sourcemap := fmt.Sprintf(
									`{ "version": 3, "sources": ["%s"], "sourcesContent": [%s], "mappings": "%s", "names": [] }`,
									transformOptions.Filename,
									string(sourcesContent),
									string(builder.GenerateChunk(output).Buffer),
								)
								script.Map = sourcemap
								script.Code = string(output)
							} else {
								src := []byte(node.FirstChild.Data)
								hoisted := js_scanner.HoistImports(src)
								output := make([]byte, 0)
								for _, statement := range hoisted.Hoisted {
									output = append(output, statement...)
								}
								output = append(output, []byte(fmt.Sprintf(`import deserialize from 'astro/client/deserialize.js'; export default (str) => (async function({%s}) {%s`, params, "\n"))...)
								output = append(output, hoisted.Body...)
								output = append(output, []byte(fmt.Sprintf(`%s})(deserialize(str))`, "\n"))...)

								script.Code = string(output)
							}
						} else if node.FirstChild != nil {
							script.Type = "inline"

							if transformOptions.SourceMap != "" {
								isLine := func(r rune) bool { return r == '\r' || r == '\n' }
								isNotLine := func(r rune) bool { return !(r == '\r' || r == '\n') }
								output := make([]byte, 0)
								builder := sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(source, len(strings.Split(source, "\n"))))
								sourcesContent, _ := json.Marshal(source)
								if len(node.FirstChild.Loc) > 0 {
									i := node.FirstChild.Loc[0].Start
									nonWS := strings.IndexFunc(node.FirstChild.Data, isNotLine)
									i += nonWS
									for _, ln := range strings.Split(strings.TrimFunc(node.FirstChild.Data, isLine), "\n") {
										content := []byte(ln)
										content = append(content, '\n')
										for j, b := range content {
											if j == 0 || !unicode.IsSpace(rune(b)) {
												builder.AddSourceMapping(loc.Loc{Start: i}, output)
											}
											output = append(output, b)
											i += 1
										}
									}
									output = append(output, '\n')
								} else {
									output = append(output, []byte(strings.TrimSpace(node.FirstChild.Data))...)
								}
								sourcemap := fmt.Sprintf(
									`{ "version": 3, "sources": ["%s"], "sourcesContent": [%s], "mappings": "%s", "names": [] }`,
									transformOptions.Filename,
									string(sourcesContent),
									string(builder.GenerateChunk(output).Buffer),
								)
								script.Map = sourcemap
								script.Code = string(output)
							} else {
								script.Code = node.FirstChild.Data
							}
						}

						// sourcemapString := createSourceMapString(source, result, transformOptions)
						// inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
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
