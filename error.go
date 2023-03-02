package go_dbx

import (
	"net/http"
)

type errorString string

func (e errorString) Error() string {
	return string(e)
}

func (e errorString) Code() int {
	switch e {
	case ErrNoRecords:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func (e errorString) Status() int {
	return e.Code()
}

const (
	ErrNoRecords errorString = "no records"
)
