package sqlx

import (
	"database/sql"
)

type Tx struct {
	tx         *sql.Tx
	driverName string
}

func (t *Tx) Commit() error {
	return t.tx.Commit()
}

func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

func (t *Tx) Table(name string) *Table {
	return &Table{
		exe:        t.tx,
		driverName: t.driverName,
		name:       name,
	}
}

func (t *Tx) Insert(record any) error {
	return t.Table(getTableName(record)).Insert(record)
}

func (t *Tx) Update(record any) error {
	return t.Table(getTableName(record)).Update(record)
}

func (t *Tx) Save(record any) error {
	return t.Table(getTableName(record)).Save(record)
}

func (t *Tx) Select(records any, where string, args ...any) error {
	return t.Table(getTableNameBySlice(records)).Select(records, where, args...)
}

func (t *Tx) SelectOne(record any, where string, args ...any) error {
	return t.Table(getTableName(record)).SelectOne(record, where, args...)
}

func (t *Tx) Exec(query string, args ...any) (sql.Result, error) {
	return t.tx.Exec(query, args...)
}
