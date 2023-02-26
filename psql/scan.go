package psql

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
)

type supportedScanTypes interface {
	*Point | *Place | *PhoneNumber | *FullName | *Money | map[string]string
}

func Scan[T supportedScanTypes](v *T) sql.Scanner {
	switch val := any(v).(type) {
	case **Point:
		return &pointScanner{v: val}
	case **PhoneNumber:
		return &phoneNumberScanner{v: val}
	case **Place:
		return &placeScanner{v: val}
	case **Money:
		return &moneyScanner{v: val}
	case **FullName:
		return &fullNameScanner{v: val}
	case *map[string]string:
		return &hstoreScanner{m: val}
	default:
		panic(fmt.Sprintf("unsupported scan type: %T", v))
	}
}

func Value[T supportedScanTypes](v T) driver.Valuer {
	switch val := any(v).(type) {
	case *Point:
		return &pointValuer{v: val}
	case *PhoneNumber:
		return &phoneNumberValuer{v: val}
	case *Place:
		return &placeValuer{v: val}
	case *Money:
		return &moneyValuer{v: val}
	case *FullName:
		return &fullNameValuer{v: val}
	case map[string]string:
		return mapToHstore(val)
	default:
		panic(fmt.Sprintf("unsupported scan type: %T", v))
	}
}
