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
	"github.com/withastro/compiler/internal/js_scanner"
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

func jsBoolOptional(j js.Value, defaultValue bool) bool {
	if j.Equal(js.Undefined()) || j.Equal(js.Null()) {
		return defaultValue
	}
	return j.Bool()
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

	filename := jsString(options.Get("filename"))
	if filename == "" {
		filename = "<stdin>"
	}

	return t.ParseOptions{
		Filename: filename,
		Position: position,
	}
}

func makeTransformOptions(options js.Value) transform.TransformOptions {
	filename := jsString(options.Get("filename"))
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

	scopedSlot := false
	if jsBool(options.Get("resultScopedSlot")) {
		scopedSlot = true
	}

	transitionsAnimationURL := jsString(options.Get("transitionsAnimationURL"))
	if transitionsAnimationURL == "" {
		transitionsAnimationURL = "astro/components/viewtransitions.css"
	}

	annotateSourceFile := false
	if jsBool(options.Get("annotateSourceFile")) {
		annotateSourceFile = true
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

	scopedStyleStrategy := jsString(options.Get("scopedStyleStrategy"))
	if scopedStyleStrategy == "" {
		scopedStyleStrategy = "where"
	}

	renderScript := false
	if jsBool(options.Get("renderScript")) {
		renderScript = true
	}

	experimentalScriptOrder := false
	if jsBool(options.Get("experimentalScriptOrder")) {
		experimentalScriptOrder = true
	}

	return transform.TransformOptions{
		Filename:                filename,
		NormalizedFilename:      normalizedFilename,
		InternalURL:             internalURL,
		SourceMap:               sourcemap,
		AstroGlobalArgs:         astroGlobalArgs,
		Compact:                 compact,
		ResolvePath:             resolvePathFn,
		PreprocessStyle:         preprocessStyle,
		ResultScopedSlot:        scopedSlot,
		ScopedStyleStrategy:     scopedStyleStrategy,
		TransitionsAnimationURL: transitionsAnimationURL,
		AnnotateSourceFile:      annotateSourceFile,
		RenderScript:            renderScript,
		ExperimentalScriptOrder: experimentalScriptOrder,
	}
}

func makeTSXOptions(options js.Value) printer.TSXOptions {
	includeScripts := jsBoolOptional(options.Get("includeScripts"), true)
	includeStyles := jsBoolOptional(options.Get("includeStyles"), true)

	return printer.TSXOptions{
		IncludeScripts: includeScripts,
		IncludeStyles:  includeStyles,
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
	LocalName    string `js:"localName"`
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
	Ranges      printer.TSXRanges       `js:"metaRanges"`
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
	ServerComponents     []HydratedComponent     `js:"serverComponents"`
	ContainsHead         bool                    `js:"containsHead"`
	StyleError           []string                `js:"styleError"`
	Propagation          bool                    `js:"propagation"`
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
		transformOptions.Scope = "xxxxxx"
		h := handler.NewHandler(source, parseOptions.Filename)

		var doc *astro.Node
		doc, err := astro.ParseWithOptions(strings.NewReader(source), astro.ParseOptionWithHandler(h), astro.ParseOptionEnableLiteral(true))
		if err != nil {
			h.AppendError(err)
		}
		result := printer.PrintToJSON(source, doc, parseOptions)

		var fmContent []byte
		if doc.FirstChild.Type == astro.FrontmatterNode && doc.FirstChild.FirstChild != nil {
			fmContent = []byte(doc.FirstChild.FirstChild.Data)
		}
		s := js_scanner.NewScanner(fmContent)

		// AFTER printing, exec transformations to pickup any errors/warnings
		transform.Transform(doc, s, transformOptions, h)

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
		transformOptions.Scope = "xxxxxx"
		h := handler.NewHandler(source, transformOptions.Filename)

		var doc *astro.Node
		doc, err := astro.ParseWithOptions(strings.NewReader(source), astro.ParseOptionWithHandler(h), astro.ParseOptionEnableLiteral(true))
		if err != nil {
			h.AppendError(err)
		}

		tsxOptions := makeTSXOptions(js.Value(args[1]))

		var fmContent []byte
		if doc.FirstChild.Type == astro.FrontmatterNode && doc.FirstChild.FirstChild != nil {
			fmContent = []byte(doc.FirstChild.FirstChild.Data)
		}
		s := js_scanner.NewScanner(fmContent)
		result := printer.PrintToTSX(source, doc, s, tsxOptions, transformOptions, h)

		// AFTER printing, exec transformations to pickup any errors/warnings
		transform.Transform(doc, s, transformOptions, h)

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
			Ranges:      result.TSXRanges,
		}).Value
	})
}

func Transform() any {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		source := strings.TrimRightFunc(jsString(args[0]), unicode.IsSpace)

		transformOptions := makeTransformOptions(js.Value(args[1]))
		scopeStr := transformOptions.NormalizedFilename
		if scopeStr == "<stdin>" {
			scopeStr = source
		}
		transformOptions.Scope = astro.HashString(scopeStr)
		h := handler.NewHandler(source, transformOptions.Filename)

		styleError := []string{}
		promiseHandle := js.FuncOf(func(this js.Value, args []js.Value) any {
			resolve := args[0]
			reject := args[1]

			go func() {
				var doc *astro.Node
				defer func() {
					if err := recover(); err != nil {
						reject.Invoke(wasm_utils.ErrorToJSError(h, err.(error)))
						return
					}
				}()

				doc, err := astro.ParseWithOptions(strings.NewReader(source), astro.ParseOptionWithHandler(h))
				if err != nil {
					reject.Invoke(wasm_utils.ErrorToJSError(h, err))
					return
				}

				// Hoist styles and scripts to the top-level
				transform.ExtractStyles(doc, &transformOptions)

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

				var fmContent []byte
				if doc.FirstChild.Type == astro.FrontmatterNode && doc.FirstChild.FirstChild != nil {
					fmContent = []byte(doc.FirstChild.FirstChild.Data)
				}
				s := js_scanner.NewScanner(fmContent)

				// Perform CSS and element scoping as needed
				transform.Transform(doc, s, transformOptions, h)

				css := []string{}
				scripts := []HoistedScript{}
				hydratedComponents := []HydratedComponent{}
				clientOnlyComponents := []HydratedComponent{}
				serverComponents := []HydratedComponent{}
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

				for _, c := range doc.ServerComponents {
					serverComponents = append(serverComponents, HydratedComponent{
						ExportName:   c.ExportName,
						LocalName:    c.LocalName,
						Specifier:    c.Specifier,
						ResolvedPath: c.ResolvedPath,
					})
				}

				var value vert.Value
				result := printer.PrintToJS(source, doc, s, len(css), transformOptions, h)
				transformResult := &TransformResult{
					CSS:                  css,
					Scope:                transformOptions.Scope,
					Scripts:              scripts,
					HydratedComponents:   hydratedComponents,
					ClientOnlyComponents: clientOnlyComponents,
					ServerComponents:     serverComponents,
					ContainsHead:         doc.ContainsHead,
					StyleError:           styleError,
					Propagation:          doc.HeadPropagation,
				}
				switch transformOptions.SourceMap {
				case "external":
					value = createExternalSourceMap(source, transformResult, result, transformOptions)
				case "both":
					value = createBothSourceMap(source, transformResult, result, transformOptions)
				case "inline":
					value = createInlineSourceMap(source, transformResult, result, transformOptions)
				default:
					transformResult.Code = string(result.Output)
					transformResult.Map = ""
					value = vert.ValueOf(transformResult)
				}
				value.Set("diagnostics", vert.ValueOf(h.Diagnostics()).Value)
				resolve.Invoke(value.Value)
			}()

			return nil
		})
		defer promiseHandle.Release()

		// Create and return the Promise object
		promiseConstructor := js.Global().Get("Promise")
		return promiseConstructor.New(promiseHandle)
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

func createExternalSourceMap(source string, transformResult *TransformResult, result printer.PrintResult, transformOptions transform.TransformOptions) vert.Value {
	transformResult.Code = string(result.Output)
	transformResult.Map = createSourceMapString(source, result, transformOptions)
	return vert.ValueOf(transformResult)
}

func createInlineSourceMap(source string, transformResult *TransformResult, result printer.PrintResult, transformOptions transform.TransformOptions) vert.Value {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	transformResult.Code = string(result.Output) + "\n" + inlineSourcemap
	transformResult.Map = ""
	return vert.ValueOf(transformResult)
}

func createBothSourceMap(source string, transformResult *TransformResult, result printer.PrintResult, transformOptions transform.TransformOptions) vert.Value {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	transformResult.Code = string(result.Output) + "\n" + inlineSourcemap
	transformResult.Map = sourcemapString
	return vert.ValueOf(transformResult)
}
