package psql

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"

	"go.olapie.com/dbx/psql/internal/composite"
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type pointScanner struct {
	v **Point
}

type pointValuer struct {
	v *Point
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

	fields, err := composite.ParseFields(s)
	if err != nil {
		return fmt.Errorf("parse composite fields %s: %w", s, err)
	}

	if len(fields) == 1 {
		fields = strings.Split(fields[0], " ")
	}

	if len(fields) != 2 {
		return fmt.Errorf("parse composite fields %s", s)
	}

	v := new(Point)
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
