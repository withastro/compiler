package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"syscall/js"
	"unicode"

	"github.com/norunners/vert"
	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/printer"
	"github.com/withastro/compiler/internal/sourcemap"
	t "github.com/withastro/compiler/internal/t"
	"github.com/withastro/compiler/internal/transform"
	wasm_utils "github.com/withastro/compiler/internal_wasm/utils"
)

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

	filename := jsString(options.Get("sourcefile"))
	if filename == "" {
		filename = "<stdin>"
	}

	return t.ParseOptions{
		Filename: filename,
		Position: position,
	}
}

func makeTransformOptions(options js.Value) transform.TransformOptions {
	filename := jsString(options.Get("sourcefile"))
	if filename == "" {
		filename = "<stdin>"
	}

	normalizedFilename := jsString(options.Get("normalizedFilename"))
	if normalizedFilename == "" {
		normalizedFilename = filename
	}

	internalURL := jsString(options.Get("internalURL"))
	if internalURL == "" {
		internalURL = "astro/runtime/server/index.js"
	}

	sourcemap := jsString(options.Get("sourcemap"))
	if sourcemap == "<boolean: true>" {
		sourcemap = "both"
	}

	astroGlobalArgs := jsString(options.Get("astroGlobalArgs"))

	compact := false
	if jsBool(options.Get("compact")) {
		compact = true
	}

	var resolvePath any = options.Get("resolvePath")
	var resolvePathFn func(string) string
	if resolvePath.(js.Value).Type() == js.TypeFunction {
		resolvePathFn = func(id string) string {
			result, _ := wasm_utils.Await(resolvePath.(js.Value).Invoke(id))
			if result[0].Equal(js.Undefined()) || result[0].Equal(js.Null()) {
				return id
			} else {
				return result[0].String()
			}
		}
	}

	preprocessStyle := options.Get("preprocessStyle")

	return transform.TransformOptions{
		Filename:           filename,
		NormalizedFilename: normalizedFilename,
		Scope:              astro.HashString(normalizedFilename),
		InternalURL:        internalURL,
		SourceMap:          sourcemap,
		AstroGlobalArgs:    astroGlobalArgs,
		Compact:            compact,
		ResolvePath:        resolvePathFn,
		PreprocessStyle:    preprocessStyle,
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
	AST         string                  `js:"ast"`
	Diagnostics []loc.DiagnosticMessage `js:"diagnostics"`
}

type TSXResult struct {
	Code        string                  `js:"code"`
	Map         string                  `js:"map"`
	Diagnostics []loc.DiagnosticMessage `js:"diagnostics"`
}

type TransformResult struct {
	Code                 string                  `js:"code"`
	Diagnostics          []loc.DiagnosticMessage `js:"diagnostics"`
	Map                  string                  `js:"map"`
	Scope                string                  `js:"scope"`
	CSS                  []string                `js:"css"`
	Scripts              []HoistedScript         `js:"scripts"`
	HydratedComponents   []HydratedComponent     `js:"hydratedComponents"`
	ClientOnlyComponents []HydratedComponent     `js:"clientOnlyComponents"`
	StyleError           []string                `js:"styleError"`
}

// This is spawned as a goroutine to preprocess style nodes using an async function passed from JS
func preprocessStyle(i int, style *astro.Node, transformOptions transform.TransformOptions, styleError *[]string, cb func()) {
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
	// If an error return, override the style's CSS so the compiler doesn't hang
	// And return a styleError. The caller will use this to know that style processing failed.
	if err := jsString(data[0].Get("error")); err != "" {
		style.FirstChild.Data = ""
		//*styleError = err
		*styleError = append(*styleError, err)
		return
	}
	str := jsString(data[0].Get("code"))
	if str == "" {
		return
	}
	style.FirstChild.Data = str
}

func Parse() any {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		source := jsString(args[0])
		parseOptions := makeParseOptions(js.Value(args[1]))
		transformOptions := makeTransformOptions(js.Value(args[1]))
		transformOptions.Scope = "XXXXXX"
		h := handler.NewHandler(source, parseOptions.Filename)

		var doc *astro.Node
		doc, err := astro.ParseWithOptions(strings.NewReader(source), astro.ParseOptionWithHandler(h), astro.ParseOptionEnableLiteral(true))
		if err != nil {
			h.AppendError(err)
		}
		result := printer.PrintToJSON(source, doc, parseOptions)

		// AFTER printing, exec transformations to pickup any errors/warnings
		transform.Transform(doc, transformOptions, h)

		return vert.ValueOf(ParseResult{
			AST:         string(result.Output),
			Diagnostics: h.Diagnostics(),
		}).Value
	})
}

func ConvertToTSX() any {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		source := jsString(args[0])
		transformOptions := makeTransformOptions(js.Value(args[1]))
		transformOptions.Scope = "XXXXXX"
		h := handler.NewHandler(source, transformOptions.Filename)

		var doc *astro.Node
		doc, err := astro.ParseWithOptions(strings.NewReader(source), astro.ParseOptionWithHandler(h), astro.ParseOptionEnableLiteral(true))
		if err != nil {
			h.AppendError(err)
		}
		result := printer.PrintToTSX(source, doc, transformOptions, h)

		// AFTER printing, exec transformations to pickup any errors/warnings
		transform.Transform(doc, transformOptions, h)

		sourcemapString := createSourceMapString(source, result, transformOptions)
		code := string(result.Output)
		if transformOptions.SourceMap != "external" {
			inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
			code += "\n" + inlineSourcemap
		}

		return vert.ValueOf(TSXResult{
			Code:        code,
			Map:         sourcemapString,
			Diagnostics: h.Diagnostics(),
		}).Value
	})
}

func Transform() any {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		source := jsString(args[0])

		transformOptions := makeTransformOptions(js.Value(args[1]))
		h := handler.NewHandler(source, transformOptions.Filename)

		styleError := []string{}
		handler := js.FuncOf(func(this js.Value, args []js.Value) any {
			resolve := args[0]

			go func() {
				var doc *astro.Node

				doc, err := astro.ParseWithOptions(strings.NewReader(source), astro.ParseOptionWithHandler(h))
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
							go preprocessStyle(i, style, transformOptions, &styleError, wg.Done)
						}
					}
				}
				// Wait for all the style goroutines to finish
				wg.Wait()

				// Perform CSS and element scoping as needed
				transform.Transform(doc, transformOptions, h)

				css := []string{}
				scripts := []HoistedScript{}
				hydratedComponents := []HydratedComponent{}
				clientOnlyComponents := []HydratedComponent{}
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
						Map:  "",
					}

					if src != nil {
						script.Type = "external"
						script.Src = src.Val
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

				var value vert.Value
				result := printer.PrintToJS(source, doc, len(css), transformOptions, h)
				switch transformOptions.SourceMap {
				case "external":
					value = createExternalSourceMap(source, result, css, &scripts, &hydratedComponents, &clientOnlyComponents, &styleError, transformOptions)
				case "both":
					value = createBothSourceMap(source, result, css, &scripts, &hydratedComponents, &clientOnlyComponents, &styleError, transformOptions)
				case "inline":
					value = createInlineSourceMap(source, result, css, &scripts, &hydratedComponents, &clientOnlyComponents, &styleError, transformOptions)
				default:
					value = vert.ValueOf(TransformResult{
						CSS:                  css,
						Code:                 string(result.Output),
						Map:                  "",
						Scope:                transformOptions.Scope,
						Scripts:              scripts,
						HydratedComponents:   hydratedComponents,
						ClientOnlyComponents: clientOnlyComponents,
						StyleError:           styleError,
					})
				}
				value.Set("diagnostics", vert.ValueOf(h.Diagnostics()).Value)
				resolve.Invoke(value.Value)
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

func createExternalSourceMap(source string, result printer.PrintResult, css []string, scripts *[]HoistedScript, hydratedComponents *[]HydratedComponent, clientOnlyComponents *[]HydratedComponent, styleError *[]string, transformOptions transform.TransformOptions) vert.Value {
	return vert.ValueOf(TransformResult{
		CSS:                  css,
		Code:                 string(result.Output),
		Map:                  createSourceMapString(source, result, transformOptions),
		Scope:                transformOptions.Scope,
		Scripts:              *scripts,
		HydratedComponents:   *hydratedComponents,
		ClientOnlyComponents: *clientOnlyComponents,
		StyleError:           *styleError,
	})
}

func createInlineSourceMap(source string, result printer.PrintResult, css []string, scripts *[]HoistedScript, hydratedComponents *[]HydratedComponent, clientOnlyComponents *[]HydratedComponent, styleError *[]string, transformOptions transform.TransformOptions) vert.Value {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	return vert.ValueOf(TransformResult{
		CSS:                  css,
		Code:                 string(result.Output) + "\n" + inlineSourcemap,
		Map:                  "",
		Scope:                transformOptions.Scope,
		Scripts:              *scripts,
		HydratedComponents:   *hydratedComponents,
		ClientOnlyComponents: *clientOnlyComponents,
		StyleError:           *styleError,
	})
}

func createBothSourceMap(source string, result printer.PrintResult, css []string, scripts *[]HoistedScript, hydratedComponents *[]HydratedComponent, clientOnlyComponents *[]HydratedComponent, styleError *[]string, transformOptions transform.TransformOptions) vert.Value {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	return vert.ValueOf(TransformResult{
		CSS:                  css,
		Code:                 string(result.Output) + "\n" + inlineSourcemap,
		Map:                  sourcemapString,
		Scope:                transformOptions.Scope,
		Scripts:              *scripts,
		HydratedComponents:   *hydratedComponents,
		ClientOnlyComponents: *clientOnlyComponents,
		StyleError:           *styleError,
	})
}
