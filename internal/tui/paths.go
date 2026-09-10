package tui

import (
	"path/filepath"
	"strings"
)

// parsePastedPaths splits drag-drop or bracketed-paste text into paths.
// Handles newline separation, shell quotes and backslash-escaped spaces.
func parsePastedPaths(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, tok := range splitShellWords(s) {
		if tok == "" || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

// splitShellWords is a tiny shell tokenizer: spaces/newlines separate, quotes
// group, backslash escapes the next rune.
func splitShellWords(s string) []string {
	var words []string
	var b strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if b.Len() > 0 {
			words = append(words, b.String())
			b.Reset()
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '\'' || r == '"' || r == '`':
			quote = r
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return words
}

// isVideoExt reports whether name has a known video extension.
func isVideoExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".mkv", ".mov", ".avi", ".webm", ".flv", ".ts", ".m4v", ".wmv":
		return true
	}
	return false
}
