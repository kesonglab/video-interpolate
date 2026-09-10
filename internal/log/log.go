// Package log wraps charm.land/log/v2 with file rotation and stderr output.
// Phase 1b will implement real rotation; for now use charm.land/log directly.
package log

import (
	"io"

	charmlog "charm.land/log/v2"
)

func New(w io.Writer, level Level) *charmlog.Logger {
	return charmlog.NewWithOptions(w, charmlog.Options{
		Level: charmlog.Level(level),
	})
}

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)
