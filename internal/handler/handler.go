package handler

import (
	"errors"
	"strings"

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
