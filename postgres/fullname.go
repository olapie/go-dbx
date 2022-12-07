package postgres

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"

	"code.olapie.com/sugar/types"
)

var (
	_ driver.Valuer = (*fullNameValuer)(nil)
	_ sql.Scanner   = (*fullNameScanner)(nil)
)

type fullNameScanner struct {
	v **types.FullName
}

// Scan implements sql.Scanner
func (fs *fullNameScanner) Scan(src any) error {
	if src == nil {
		return nil
	}

	s, ok := src.(string)
	if !ok {
		var b []byte
		b, ok = src.([]byte)
		if ok {
			s = string(b)
		}
	}

	if !ok || len(s) < 4 {
		return fmt.Errorf("failed to parse %v into sql.FullName", src)
	}

	s = s[1 : len(s)-1]
	segments := strings.Split(s, ",")
	if len(segments) != 3 {
		return fmt.Errorf("failed to parse %v into sql.FullName", src)
	}

	n := new(types.FullName)
	n.First, n.Middle, n.Last = segments[0], segments[1], segments[2]
	*fs.v = n
	return nil
}

type fullNameValuer struct {
	v *types.FullName
}

// Value implements driver.Valuer
func (fv *fullNameValuer) Value() (driver.Value, error) {
	if fv == nil || fv.v == nil {
		return nil, nil
	}
	s := fmt.Sprintf("(%s,%s,%s)", fv.v.First, fv.v.Middle, fv.v.Last)
	return s, nil
}
