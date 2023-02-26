package psql

import (
	"database/sql"
	"database/sql/driver"
	"fmt"

	"go.olapie.com/conv"
	"go.olapie.com/dbx/psql/internal/composite"
)

var (
	_ driver.Valuer = (*phoneNumberValuer)(nil)
	_ sql.Scanner   = (*phoneNumberScanner)(nil)
)

type PhoneNumber struct {
	Code   int32 `json:"code,omitempty"`
	Number int64 `json:"number,omitempty"`
}

type phoneNumberScanner struct {
	v **PhoneNumber
}

func (ps *phoneNumberScanner) Scan(src any) error {
	if src == nil {
		return nil
	}

	s, err := conv.ToString(src)
	if err != nil {
		return fmt.Errorf("parse string: %w", err)
	}
	if len(s) == 0 {
		return nil
	}

	fields, err := composite.ParseFields(s)
	if err != nil {
		return fmt.Errorf("parse composite fields %s: %w", s, err)
	}

	if len(fields) != 3 {
		return fmt.Errorf("parse composite fields %s: got %v", s, fields)
	}

	n := new(PhoneNumber)
	n.Code, err = conv.ToInt32(fields[0])
	if err != nil {
		return fmt.Errorf("parse code %s: %w", fields[0], err)
	}
	n.Number, err = conv.ToInt64(fields[1])
	if err != nil {
		return fmt.Errorf("parse code %s: %w", fields[1], err)
	}
	//n.Extension = fields[2]
	*ps.v = n
	return nil
}

type phoneNumberValuer struct {
	v *PhoneNumber
}

func (pv *phoneNumberValuer) Value() (driver.Value, error) {
	if pv == nil {
		return nil, nil
	}
	//ext := strings.Replace(pv.v.Extension, ",", "\\,", -1)
	ext := ""
	s := fmt.Sprintf("(%d,%d,%s)", pv.v.Code, pv.v.Number, ext)
	return s, nil
}
