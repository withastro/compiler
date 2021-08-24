package astro_wasm

import (
	"encoding/base32"
	"strings"
	"syscall/js"

	tycho "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/transform"
	"github.com/snowpackjs/astro/internal/xxhash"
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

func Transform(this js.Value, args []js.Value) interface{} {
	source := jsString(args[0])
	options := js.Value(args[1])
	doc, _ := tycho.Parse(strings.NewReader(source))
	hash := hashFromSource(source)

	transform.Transform(doc, transform.TransformOptions{
		Scope: hash,
	})

	w := new(strings.Builder)
	tycho.Render(source, doc)
	js := w.String()
	internalURL := jsString(options.Get("internalURL"))
	if internalURL == "" {
		internalURL = "@astrojs/compiler/internal"
	}

	return "import \"" + internalURL + "\";\n" + js
}

func hashFromSource(source string) string {
	h := xxhash.New()
	h.Write([]byte(source))
	hashBytes := h.Sum(nil)
	return base32.StdEncoding.EncodeToString(hashBytes)[:8]
}
