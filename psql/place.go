package psql

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"go.olapie.com/dbx/psql/internal/composite"
)

var (
	_ driver.Valuer = (*placeValuer)(nil)
	_ sql.Scanner   = (*placeScanner)(nil)
)

type Place struct {
	Code       string `json:"code,omitempty"`
	Name       string `json:"name,omitempty"`
	Coordinate *Point `json:"coordinate,omitempty"`
}

type placeScanner struct {
	v **Place
}

func (ps *placeScanner) Scan(src any) error {
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
	if len(fields) != 3 {
		return fmt.Errorf("parse composite fields %s", s)
	}
	p := new(Place)
	p.Code = fields[0]
	p.Name = fields[1]
	if len(fields[2]) > 0 {
		p.Coordinate = new(Point)
		point := pointScanner{
			v: &p.Coordinate,
		}
		if err := point.Scan(fields[2]); err != nil {
			return fmt.Errorf("scan place.Coordinate: %w", err)
		}
	}
	*ps.v = p
	return nil
}

type placeValuer struct {
	v *Place
}

func (pv *placeValuer) Value() (driver.Value, error) {
	if pv == nil || (pv.v.Code == "" && pv.v.Name == "" && pv.v.Coordinate == nil) {
		return nil, nil
	}
	point := pointValuer{
		v: pv.v.Coordinate,
	}
	loc, err := point.Value()
	if err != nil {
		return nil, fmt.Errorf("get Coordinate value: %w", err)
	}
	fields := []string{pv.v.Code, pv.v.Name}
	locStr, _ := loc.(string)
	fields = append(fields, locStr)
	return composite.FieldsToString(fields), nil
}
