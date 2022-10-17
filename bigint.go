package sqlx

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"math/big"

	"code.olapie.com/conv"
)

type BigInt big.Int

var (
	_ driver.Valuer = (*BigInt)(nil)
	_ sql.Scanner   = (*BigInt)(nil)
)

func (i *BigInt) Scan(src any) error {
	if src == nil {
		return nil
	}

	s, err := conv.ToString(src)
	if err != nil {
		return fmt.Errorf("cannot parse %v into big.Int", src)
	}

	_, ok := (*big.Int)(i).SetString(s, 10)
	if !ok {
		return fmt.Errorf("cannot parse %v into big.Int", src)
	}
	return nil
}

func (i *BigInt) Value() (driver.Value, error) {
	if i == nil {
		return nil, nil
	}
	return (*big.Int)(i).String(), nil
}

func (i *BigInt) Unwrap() *big.Int {
	return (*big.Int)(i)
}
