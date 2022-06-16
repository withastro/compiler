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
}

func NewHandler(sourcetext string, filename string) *Handler {
	return &Handler{
		sourcetext: sourcetext,
		filename:   filename,
		builder:    sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
		errors:     make([]error, 0),
		warnings:   make([]error, 0),
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

func (h *Handler) Errors() []loc.Message {
	msgs := make([]loc.Message, 0)
	for _, err := range h.errors {
		if err != nil {
			msgs = append(msgs, ErrorToMessage(h, err))
		}
	}
	return msgs
}

func (h *Handler) Warnings() []loc.Message {
	msgs := make([]loc.Message, 0)
	for _, err := range h.warnings {
		if err != nil {
			msgs = append(msgs, ErrorToMessage(h, err))
		}
	}
	return msgs
}

func ErrorToMessage(h *Handler, err error) loc.Message {
	var rangedError *loc.ErrorWithRange
	switch {
	case errors.As(err, &rangedError):
		pos := h.builder.GetLineAndColumnForLocation(rangedError.Range.Loc)
		location := &loc.MessageLocation{
			File:       h.filename,
			Line:       pos[0],
			Column:     pos[1],
			Length:     rangedError.Range.Len,
			LineText:   h.sourcetext[rangedError.Range.Loc.Start:rangedError.Range.End()],
			Suggestion: rangedError.Suggestion,
		}
		return rangedError.ToMessage(location)
	default:
		return loc.Message{Text: err.Error()}
	}
}
