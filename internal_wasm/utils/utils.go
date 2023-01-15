package wasm_utils

import (
	"fmt"
	"regexp"
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

var FN_NAME_RE = regexp.MustCompile(`(\w+)\([^)]+\)$`)

func ErrorToJSError(h *handler.Handler, err error) js.Value {
	stack := string(debug.Stack())
	message := strings.TrimSpace(err.Error())
	if strings.Contains(message, ":") {
		message = strings.TrimSpace(strings.Split(message, ":")[1])
	}
	hasFnName := false
	message = fmt.Sprintf("UnknownCompilerError: %s", message)
	cleanStack := message
	for _, v := range strings.Split(stack, "\n") {
		matches := FN_NAME_RE.FindAllString(v, -1)
		if len(matches) > 0 {
			name := strings.Split(matches[0], "(")[0]
			if name == "panic" {
				cleanStack = message
				continue
			}
			cleanStack += fmt.Sprintf("\n    at %s", strings.Split(matches[0], "(")[0])
			hasFnName = true
		} else if hasFnName {
			url := strings.Split(strings.Split(strings.TrimSpace(v), " ")[0], "/compiler/")[1]
			cleanStack += fmt.Sprintf(" (@astrojs/compiler/%s)", url)
			hasFnName = false
		}
	}
	jsError := JSError{
		Message: message,
		Stack:   cleanStack,
	}
	return jsError.Value()
}
