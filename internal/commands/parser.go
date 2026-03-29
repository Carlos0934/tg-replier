package commands

import "errors"

// tokenize splits a command string into tokens on whitespace, respecting
// single- and double-quoted spans. Surrounding quotes are stripped from
// quoted tokens. Escaped quotes (\" inside double-quoted, \' inside
// single-quoted) are treated as literal quote characters. An empty
// quoted string ("" or ”) produces an empty-string token.
//
// Returns (nil, error) when the input contains an unmatched quote.
func tokenize(text string) ([]string, error) {
	var tokens []string
	var buf []rune

	const (
		normal   = iota
		inDouble // inside "…"
		inSingle // inside '…'
	)
	state := normal

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		switch state {
		case normal:
			switch {
			case ch == '"':
				state = inDouble
			case ch == '\'':
				state = inSingle
			case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
				if len(buf) > 0 {
					tokens = append(tokens, string(buf))
					buf = buf[:0]
				}
			default:
				buf = append(buf, ch)
			}

		case inDouble:
			if ch == '\\' && i+1 < len(runes) && runes[i+1] == '"' {
				buf = append(buf, '"')
				i++ // skip escaped quote
			} else if ch == '"' {
				// Close the double-quoted span; flush token (may be empty).
				tokens = append(tokens, string(buf))
				buf = buf[:0]
				state = normal
			} else {
				buf = append(buf, ch)
			}

		case inSingle:
			if ch == '\\' && i+1 < len(runes) && runes[i+1] == '\'' {
				buf = append(buf, '\'')
				i++ // skip escaped quote
			} else if ch == '\'' {
				tokens = append(tokens, string(buf))
				buf = buf[:0]
				state = normal
			} else {
				buf = append(buf, ch)
			}
		}
	}

	if state != normal {
		return nil, errors.New("malformed command: unmatched quote")
	}

	// Flush any trailing unquoted token.
	if len(buf) > 0 {
		tokens = append(tokens, string(buf))
	}

	return tokens, nil
}
