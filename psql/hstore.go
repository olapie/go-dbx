package psql

import (
	"database/sql"

	"github.com/lib/pq/hstore"
)

func mapToHstore(m map[string]string) hstore.Hstore {
	var h hstore.Hstore
	for k, v := range m {
		h.Map[k] = sql.NullString{String: v}
	}
	return h
}

func hstoreToMap(h hstore.Hstore) map[string]string {
	m := make(map[string]string, len(h.Map))
	for k, v := range h.Map {
		if v.Valid {
			m[k] = v.String
		}
	}
	return m
}

type hstoreScanner struct {
	m *map[string]string
}

var _ sql.Scanner = (*hstoreScanner)(nil)

func (hs *hstoreScanner) Scan(src any) error {
	if src == nil {
		return nil
	}
	var h hstore.Hstore
	err := h.Scan(src)
	if err != nil {
		return err
	}
	m := hstoreToMap(h)
	*hs.m = m
	return nil
}
