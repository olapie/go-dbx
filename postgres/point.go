package postgres

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"

	"code.olapie.com/sugar/types"
)

type pointScanner struct {
	v **types.Point
}

type pointValuer struct {
	v *types.Point
}

var (
	_ driver.Valuer = (*pointValuer)(nil)
	_ sql.Scanner   = (*pointScanner)(nil)
)

func (p *pointScanner) Scan(src any) error {
	if src == nil {
		return nil
	}

	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("cannot parse %v into string", src)
	}

	if s == "" {
		return nil
	}

	fields, err := ParseCompositeFields(s)
	if err != nil {
		return fmt.Errorf("parse composite fields %s: %w", s, err)
	}

	if len(fields) == 1 {
		fields = strings.Split(fields[0], " ")
	}

	if len(fields) != 2 {
		return fmt.Errorf("parse composite fields %s", s)
	}

	v := new(types.Point)
	_, err = fmt.Sscanf(fields[0], "%f", &v.X)
	if err != nil {
		return fmt.Errorf("parse x %s: %w", fields[0], err)
	}
	_, err = fmt.Sscanf(fields[1], "%f", &v.Y)
	if err != nil {
		return fmt.Errorf("parse y %s: %w", fields[1], err)
	}
	*p.v = v
	return nil
}

func (p *pointValuer) Value() (driver.Value, error) {
	if p == nil || p.v == nil {
		return nil, nil
	}
	v := fmt.Sprintf("POINT(%f %f)", p.v.X, p.v.Y)
	return v, nil
}
