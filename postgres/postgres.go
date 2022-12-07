package postgres

import (
	"database/sql"
	"database/sql/driver"

	"code.olapie.com/sugar/types"
)

type supportedTypes interface {
	*types.Point | *types.Place | *types.PhoneNumber | *types.FullName | *types.Money | map[string]string
}

func Scan[T supportedTypes](v *T) sql.Scanner {
	switch val := any(v).(type) {
	case **types.Point:
		return &pointScanner{v: val}
	case **types.PhoneNumber:
		return &phoneNumberScanner{v: val}
	case **types.Place:
		return &placeScanner{v: val}
	case **types.Money:
		return &moneyScanner{v: val}
	case **types.FullName:
		return &fullNameScanner{v: val}
	case *map[string]string:
		return &hstoreScanner{m: val}
	default:
		panic("unsupported type")
	}
}

func Value[T supportedTypes](v T) driver.Valuer {
	switch val := any(v).(type) {
	case *types.Point:
		return &pointValuer{v: val}
	case *types.PhoneNumber:
		return &phoneNumberValuer{v: val}
	case *types.Place:
		return &placeValuer{v: val}
	case *types.Money:
		return &moneyValuer{v: val}
	case *types.FullName:
		return &fullNameValuer{v: val}
	case map[string]string:
		return mapToHstore(val)
	default:
		panic("unsupported type")
	}
}
