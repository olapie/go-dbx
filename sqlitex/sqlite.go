package sqlitex

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func Open(fileName string) (*sql.DB, error) {
	dirname := filepath.Dir(fileName)
	if fi, err := os.Stat(dirname); err != nil {
		if err == os.ErrNotExist {
			err := os.MkdirAll(dirname, 0755)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else if !fi.IsDir() {
		return nil, errors.New(dirname + " is not a directory")
	}

	_, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	dataSource := fmt.Sprintf("file:%s?cache=shared", fileName)
	return sql.Open("sqlite3", dataSource)
}

func MustOpen(filename string) *sql.DB {
	return MustGet(Open(filename))
}

// MustGet eliminates nil err and panics if err isn't nil
func MustGet[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
