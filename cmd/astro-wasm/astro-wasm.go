package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	internalURL := jsString(options.Get("internalURL"))
	if internalURL == "" {
		internalURL = "astro/internal"
	}

	return transform.TransformOptions{
		Scope:       hash,
		InternalURL: internalURL,
	}
}

func Transform(this js.Value, args []js.Value) interface{} {
	source := jsString(args[0])

	doc, _ := astro.Parse(strings.NewReader(source))
	hash := astro.HashFromSource(source)
	transformOptions := makeTransformOptions(js.Value(args[1]), hash)
	fmt.Println(transformOptions.InternalURL)

	transform.Transform(doc, transformOptions)

	result := printer.PrintToJS(source, doc, transformOptions)
	content, _ := json.Marshal(source)
	sourcemap := `{ "version": 3, "sources": ["file.astro"], "names": [], "mappings": "` + string(result.SourceMapChunk.Buffer) + `", "sourcesContent": [` + string(content) + `] }`
	b64 := base64.StdEncoding.EncodeToString([]byte(sourcemap))
	output := string(result.Output) + string('\n') + `//# sourceMappingURL=data:application/json;base64,` + b64 + string('\n')

	return output
}
