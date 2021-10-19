package transform

import (
	"fmt"
	"syscall/js"

	astro "github.com/snowpackjs/astro/internal"
)

func Preprocess(n *astro.Node) {
	globalThis := js.Global()
	stylePreprocess := globalThis.Get("__astro_stylePreprocess")
	if stylePreprocess.IsUndefined() == true {
		return
	}

	// var newValue js.Value

	dataOrPromise := stylePreprocess.Invoke(n.FirstChild.Data, getQuotedAttr(n, "lang"))
	str := dataOrPromise.String()

	fmt.Println(str)
	// if str != "" {
	// 	res, _ := wasm_utils.Await(dataOrPromise)
	// 	newValue = res[0]
	// } else {
	// 	newValue = dataOrPromise
	// }

	n.FirstChild.Data = str
}
