package wasm_utils

import (
	"syscall/js"

	astro "github.com/snowpackjs/astro/internal"
)

// See https://stackoverflow.com/questions/68426700/how-to-wait-a-js-async-function-from-golang-wasm
func Await(awaitable js.Value) ([]js.Value, []js.Value) {
	then := make(chan []js.Value)
	thenFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		then <- args
		return nil
	})
	// defers are called LIFO!
	// This will `close` before `Release()`
	defer thenFunc.Release()
	defer close(then)

	catch := make(chan []js.Value)
	catchFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		catch <- args
		return nil
	})
	// defers are called LIFO!
	// This will `close` before `Release()`
	defer catchFunc.Release()
	defer close(catch)

	awaitable.Call("then", thenFunc).Call("catch", catchFunc)

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
