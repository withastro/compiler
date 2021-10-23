package wasm_utils

import (
	"syscall/js"

	astro "github.com/snowpackjs/astro/internal"
)

// See https://stackoverflow.com/questions/68426700/how-to-wait-a-js-async-function-from-golang-wasm
func Await(awaitable js.Value) ([]js.Value, []js.Value) {
	then := make(chan []js.Value)
	defer close(then)
	thenFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		then <- args
		return nil
	})

	catch := make(chan []js.Value)
	defer close(catch)
	catchFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		catch <- args
		return nil
	})

	awaitable.Call("then", thenFunc).Call("catch", catchFunc)

	// only release after awaitable.Call
	defer thenFunc.Release()
	defer catchFunc.Release()

	select {
	case result := <-then:
		return result, nil
	case err := <-catch:
		return nil, err
	}
}

func GetAttrs(n *astro.Node) js.Value {
	attrs := js.Global().Get("Object").New()
	for _, attr := range n.Attr {
		switch attr.Type {
		case astro.QuotedAttribute:
			attrs.Set(attr.Key, attr.Val)
		case astro.EmptyAttribute:
			attrs.Set(attr.Key, true)
		}
	}
	return attrs
}
