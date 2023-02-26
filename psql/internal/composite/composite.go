package composite

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

type parsingState int

const (
	parsingInit parsingState = iota
	parsingField
	parsingQuoted
)

func ParseFields(column string) ([]string, error) {
	if column == "" {
		return nil, errors.New("empty column")
	}

	fields := make([]string, 0, 2)
	state := parsingInit
	var field bytes.Buffer
	chars := []rune(column)
	n := len(chars)
	errPos := -1
Loop:
	for i := 0; i < n; i++ {
		c := chars[i]
		switch state {
		case parsingInit:
			if c != '(' {
				//errPos = i
				//break Loop
				continue
			}
			state = parsingField
		case parsingField:
			switch c {
			case '"':
				if field.Len() == 0 {
					state = parsingQuoted
				} else {
					if i == len(chars)-1 || chars[i+1] != '"' {
						errPos = i
						break Loop
					}
					field.WriteRune('"')
					i++
				}
			case ')':
				fields = append(fields, field.String())
				if i != len(chars)-1 {
					errPos = i
					break Loop
				}
				return fields, nil
			case ',':
				fields = append(fields, field.String())
				field.Reset()
			default:
				field.WriteRune(c)
			}
		case parsingQuoted:
			switch c {
			case '"':
				if i == len(chars)-1 {
					errPos = i
					break Loop
				}
				i++
				switch chars[i] {
				case '"':
					// In quoted string, "" represents "
					field.WriteRune('"')
				case ',':
					fields = append(fields, field.String())
					field.Reset()
					state = parsingField
				case ')':
					fields = append(fields, field.String())
					if i != len(chars)-1 {
						errPos = i
						break Loop
					}
					return fields, nil
				default:
					errPos = i
					break Loop
				}
			default:
				field.WriteRune(c)
			}
		}
	}
	return nil, fmt.Errorf("syntax error at %d", errPos)
}

func FieldsToString(fields ...any) string {
	var builder strings.Builder
	builder.WriteString("(")
	n := len(fields)
	for i, field := range fields {
		switch v := field.(type) {
		case string:
			builder.WriteString(EscapeField(v))
		case []byte:
			builder.WriteString(EscapeField(string(v)))
		default:
			builder.WriteString(EscapeField(fmt.Sprint(v)))
		}
		if i < n-1 {
			builder.WriteRune(',')
		}
	}
	builder.WriteString(")")
	return builder.String()
}

func EscapeField(s string) string {
	s = strings.Replace(s, ",", "\\,", -1)
	s = strings.Replace(s, "(", "\\(", -1)
	s = strings.Replace(s, ")", "\\)", -1)
	s = strings.Replace(s, "\"", "\\\"", -1)
	return s
}
