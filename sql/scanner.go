// Copyright © 2016 Abcum Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sql

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"
)

// Scanner represents a lexical scanner.
type Scanner struct {
	r *bufio.Reader
}

// NewScanner returns a new instance of Scanner.
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{r: bufio.NewReader(r)}
}

// Scan returns the next token and literal value.
func (s *Scanner) Scan() (tok Token, lit string) {

	// Read the next rune.
	ch := s.read()

	// If we see whitespace then consume all contiguous whitespace.
	if isWhitespace(ch) {
		s.unread()
		return s.scanWhitespace()
	}

	// If we see a letter then consume as an string.
	if isLetter(ch) {
		s.unread()
		return s.scanIdent()
	}

	// If we see a number then consume as a number.
	if isNumber(ch) {
		s.unread()
		return s.scanNumber()
	}

	// Otherwise read the individual character.
	switch ch {

	case eof:
		return EOF, ""
	case '*':
		return ALL, string(ch)
	case '@':
		return EAT, string(ch)
	case ',':
		return COMMA, string(ch)
	case '.':
		chn := s.read()
		s.unread()
		if isNumber(chn) {
			return s.scanNumber()
		}
		return DOT, string(ch)
	case '"':
		s.unread()
		return s.scanString()
	case '\'':
		s.unread()
		return s.scanString()
	case '`':
		s.unread()
		return s.scanQuoted()
	case '{':
		s.unread()
		return s.scanSpecial()
	case '[':
		s.unread()
		return s.scanSpecial()
	case ':':
		return COLON, string(ch)
	case ';':
		return SEMICOLON, string(ch)
	case '(':
		return LPAREN, string(ch)
	case ')':
		return RPAREN, string(ch)
	case '=':
		return EQ, string(ch)
	case '+':
		if chn := s.read(); chn == '=' {
			return INC, "+="
		}
		s.unread()
		return ADD, string(ch)
	case '-':
		if chn := s.read(); chn == '=' {
			return DEC, "-="
		}
		s.unread()
		return SUB, string(ch)
	case '!':
		if chn := s.read(); chn == '=' {
			return NEQ, "!="
		}
		s.unread()
	case '<':
		if chn := s.read(); chn == '=' {
			return LTE, "<="
		}
		s.unread()
		return LT, string(ch)
	case '>':
		if chn := s.read(); chn == '=' {
			return GTE, ">="
		}
		s.unread()
		return GT, string(ch)

	}

	return ILLEGAL, string(ch)

}

// scanWhitespace consumes the current rune and all contiguous whitespace.
func (s *Scanner) scanWhitespace() (tok Token, lit string) {

	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent whitespace character into the buffer.
	// Non-whitespace characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isWhitespace(ch) {
			s.unread()
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	return WS, buf.String()

}

// scanIdent consumes the current rune and all contiguous ident runes.
func (s *Scanner) scanIdent() (tok Token, lit string) {

	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent ident character into the buffer.
	// Non-ident characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isIdentChar(ch) {
			s.unread()
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	// If the string matches a keyword then return that keyword.
	if tok := keywords[strings.ToUpper(buf.String())]; tok > 0 {
		return tok, buf.String()
	}

	// Otherwise return as a regular identifier.
	return IDENT, buf.String()

}

func (s *Scanner) scanQuoted() (Token, string) {

	tok, lit := s.scanString()

	if is(tok, STRING) {
		return IDENT, lit
	}

	return tok, lit

}

func (s *Scanner) scanString() (tok Token, lit string) {

	tok = STRING

	var buf bytes.Buffer

	char := s.read()

	for {
		if ch := s.read(); ch == char {
			break
		} else if ch == eof {
			return ILLEGAL, buf.String()
		} else if ch == '\n' {
			tok = REGION
			buf.WriteRune(ch)
		} else if ch == '\\' {
			chn := s.read()
			if chn == 'n' {
				buf.WriteRune('\n')
			} else if chn == '\\' {
				buf.WriteRune('\\')
			} else if chn == '"' {
				buf.WriteRune('"')
			} else if chn == '\'' {
				buf.WriteRune('\'')
			} else if chn == '`' {
				buf.WriteRune('`')
			} else {
				return ILLEGAL, buf.String()
			}
		} else {
			buf.WriteRune(ch)
		}
	}

	return tok, buf.String()

}

func (s *Scanner) scanNumber() (tok Token, lit string) {

	tok = NUMBER

	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent ident character into the buffer.
	// Non-ident characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if ch == '.' {
			if tok == DOUBLE {
				tok = IDENT
			}
			if tok == NUMBER {
				tok = DOUBLE
			}
			buf.WriteRune(ch)
		} else if ch == '+' {
			buf.WriteRune(ch)
		} else if ch == '-' {
			buf.WriteRune(ch)
		} else if ch == ':' {
			buf.WriteRune(ch)
		} else if ch == 'T' {
			buf.WriteRune(ch)
		} else if isLetter(ch) {
			tok = IDENT
			buf.WriteRune(ch)
		} else if !isNumber(ch) {
			s.unread()
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	if _, err := time.Parse("2006-01-02", buf.String()); err == nil {
		return DATE, buf.String()
	}

	if _, err := time.Parse(time.RFC3339, buf.String()); err == nil {
		return TIME, buf.String()
	}

	if _, err := time.Parse(time.RFC3339Nano, buf.String()); err == nil {
		return NANO, buf.String()
	}

	return tok, buf.String()

}

func (s *Scanner) scanSpecial() (tok Token, lit string) {

	tok = IDENT

	var buf bytes.Buffer

	beg := s.read()
	end := beg

	if beg == '{' {
		end = '}'
	}

	if beg == '[' {
		end = ']'
	}

	for {
		if ch := s.read(); ch == end {
			break
		} else if ch == eof {
			return ILLEGAL, buf.String()
		} else if ch == '\n' {
			tok = REGION
			buf.WriteRune(ch)
		} else if ch == '\\' {
			chn := s.read()
			if chn == 'n' {
				tok = REGION
				buf.WriteRune('\n')
			} else if chn == '\\' {
				buf.WriteRune('\\')
			} else if chn == '"' {
				buf.WriteRune('"')
			} else if chn == '\'' {
				buf.WriteRune('\'')
			} else if chn == '`' {
				buf.WriteRune('`')
			} else {
				break
			}
		} else {
			buf.WriteRune(ch)
		}
	}

	var f interface{}
	j := []byte(string(beg) + buf.String() + string(end))
	err := json.Unmarshal(j, &f)
	if err == nil {
		return JSON, string(beg) + buf.String() + string(end)
	}

	return tok, buf.String()

}

// read reads the next rune from the bufferred reader.
// Returns the rune(0) if an error occurs (or io.EOF is returned).
func (s *Scanner) read() rune {
	ch, _, err := s.r.ReadRune()
	if err != nil {
		return eof
	}
	return ch
}

// unread places the previously read rune back on the reader.
func (s *Scanner) unread() {
	_ = s.r.UnreadRune()
}

func number(lit string) (i int64) {
	i, _ = strconv.ParseInt(lit, 10, 64)
	return
}

// isWhitespace returns true if the rune is a space, tab, or newline.
func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

// isLetter returns true if the rune is a letter.
func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

// isNumber returns true if the rune is a number.
func isNumber(ch rune) bool {
	return (ch >= '0' && ch <= '9')
}

// isSeparator returns true if the rune is a separator expression.
func isSeparator(ch rune) bool {
	return (ch == '.')
}

// isIdentChar returns true if the rune can be used in an unquoted identifier.
func isIdentChar(ch rune) bool {
	return isLetter(ch) || isNumber(ch) || isSeparator(ch) || ch == '_'
}

// eof represents a marker rune for the end of the reader.
var eof = rune(0)