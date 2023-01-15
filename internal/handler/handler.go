package handler

import (
	"errors"
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
	"syscall/js"

	"github.com/norunners/vert"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/sourcemap"
)

type Handler struct {
	sourcetext string
	filename   string
	builder    sourcemap.ChunkBuilder
	errors     []error
	warnings   []error
	infos      []error
	hints      []error
}

type JSError struct {
	Message string `js:"message"`
	Stack   string `js:"stack"`
}

func (err *JSError) Value() js.Value {
	return vert.ValueOf(err).Value
}

func NewHandler(sourcetext string, filename string) *Handler {
	return &Handler{
		sourcetext: sourcetext,
		filename:   filename,
		builder:    sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
		errors:     make([]error, 0),
		warnings:   make([]error, 0),
		infos:      make([]error, 0),
		hints:      make([]error, 0),
	}
}

func (h *Handler) HasErrors() bool {
	return len(h.errors) > 0
}

func (h *Handler) AppendError(err error) {
	h.errors = append(h.errors, err)
}

func (h *Handler) AppendWarning(err error) {
	h.warnings = append(h.warnings, err)
}

func (h *Handler) AppendInfo(err error) {
	h.infos = append(h.infos, err)
}
func (h *Handler) AppendHint(err error) {
	h.hints = append(h.hints, err)
}

func (h *Handler) Errors() []loc.DiagnosticMessage {
	msgs := make([]loc.DiagnosticMessage, 0)
	for _, err := range h.errors {
		if err != nil {
			msgs = append(msgs, ErrorToMessage(h, loc.ErrorType, err))
		}
	}
	return msgs
}

func (h *Handler) Warnings() []loc.DiagnosticMessage {
	msgs := make([]loc.DiagnosticMessage, 0)
	for _, err := range h.warnings {
		if err != nil {
			msgs = append(msgs, ErrorToMessage(h, loc.WarningType, err))
		}
	}
	return msgs
}

func (h *Handler) Diagnostics() []loc.DiagnosticMessage {
	msgs := make([]loc.DiagnosticMessage, 0)
	for _, err := range h.errors {
		if err != nil {
			msgs = append(msgs, ErrorToMessage(h, loc.ErrorType, err))
		}
	}
	for _, err := range h.warnings {
		if err != nil {
			msgs = append(msgs, ErrorToMessage(h, loc.WarningType, err))
		}
	}
	for _, err := range h.infos {
		if err != nil {
			msgs = append(msgs, ErrorToMessage(h, loc.InformationType, err))
		}
	}
	for _, err := range h.hints {
		if err != nil {
			msgs = append(msgs, ErrorToMessage(h, loc.HintType, err))
		}
	}
	return msgs
}

func ErrorToMessage(h *Handler, severity loc.DiagnosticSeverity, err error) loc.DiagnosticMessage {
	var rangedError *loc.ErrorWithRange
	switch {
	case errors.As(err, &rangedError):
		pos := h.builder.GetLineAndColumnForLocation(rangedError.Range.Loc)
		location := &loc.DiagnosticLocation{
			File:   h.filename,
			Line:   pos[0],
			Column: pos[1],
			Length: rangedError.Range.Len,
		}
		message := rangedError.ToMessage(location)
		message.Severity = int(severity)
		return message
	default:
		return loc.DiagnosticMessage{Text: err.Error()}
	}
}

var FN_NAME_RE = regexp.MustCompile(`(\w+)\([^)]+\)$`)

func ErrorToJSError(h *Handler, err error) js.Value {
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
