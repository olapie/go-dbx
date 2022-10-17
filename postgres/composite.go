package postgres

import (
	"bytes"
	"fmt"
)

type compositeScanState int

const (
	_COMPOSITE_SCAN_INIT compositeScanState = iota
	_COMPOSITE_SCAN_FIELD
	_COMPOSITE_SCAN_QUOTED
)

func ParseCompositeFields(column string) ([]string, error) {
	if len(column) == 0 {
		return nil, fmt.Errorf("empty column")
	}

	fields := make([]string, 0, 2)
	state := _COMPOSITE_SCAN_INIT
	var field bytes.Buffer
	chars := []rune(column)
	n := len(chars)
	errPos := -1
Loop:
	for i := 0; i < n; i++ {
		c := chars[i]
		switch state {
		case _COMPOSITE_SCAN_INIT:
			if c != '(' {
				//errPos = i
				//break Loop
				continue
			}
			state = _COMPOSITE_SCAN_FIELD
		case _COMPOSITE_SCAN_FIELD:
			switch c {
			case '"':
				if field.Len() == 0 {
					state = _COMPOSITE_SCAN_QUOTED
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
		case _COMPOSITE_SCAN_QUOTED:
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
					state = _COMPOSITE_SCAN_FIELD
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
