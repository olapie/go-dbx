package postgres

import (
	"database/sql"
	"database/sql/driver"
	"fmt"

	"code.olapie.com/sugar/types"
)

var (
	_ driver.Valuer = (*moneyValuer)(nil)
	_ sql.Scanner   = (*moneyScanner)(nil)
)

type moneyScanner struct {
	v **types.Money
}

func (ms *moneyScanner) Scan(src any) error {
	if src == nil {
		return nil
	}

	s, ok := src.(string)
	if !ok {
		b, ok := src.([]byte)
		if !ok {
			return fmt.Errorf("src is not []byte or string")
		}
		s = string(b)
	}

	if len(s) == 0 {
		return nil
	}

	fields, err := ParseCompositeFields(s)
	if err != nil {
		return fmt.Errorf("parse composite fields %s: %w", s, err)
	}

	if len(fields) != 2 {
		return fmt.Errorf("parse composite fields %s: got %v", s, fields)
	}
	m := new(types.Money)
	m.Currency = fields[0]
	m.Amount = fields[1]
	if err != nil {
		return fmt.Errorf("parse amount %s: %w", fields[1], err)
	}
	*ms.v = m
	return nil
}

type moneyValuer struct {
	v *types.Money
}

func (mv *moneyValuer) Value() (driver.Value, error) {
	if mv == nil || mv.v == nil {
		return nil, nil
	}
	return fmt.Sprintf("(%s,%s)", mv.v.Currency, mv.v.Amount), nil
}
