//go:build js && wasm

package wasm_utils

import (
	"runtime/debug"
	"strings"
	"syscall/js"

	"github.com/norunners/vert"
	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
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

type JSError struct {
	Message string `js:"message"`
	Stack   string `js:"stack"`
}

func (err *JSError) Value() js.Value {
	return vert.ValueOf(err).Value
}

func ErrorToJSError(h *handler.Handler, err error) js.Value {
	stack := string(debug.Stack())
	message := strings.TrimSpace(err.Error())
	jsError := JSError{
		Message: message,
		Stack:   stack,
	}
	return jsError.Value()
}
