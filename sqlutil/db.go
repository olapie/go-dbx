package sqlutil

import (
	"database/sql"
	"errors"
	"reflect"
)

var _tableNamerType = reflect.TypeOf((*tableNamer)(nil)).Elem()

type DB struct {
	db         *sql.DB
	driverName string
}

// NewDB opens database
// dataSourceName's format: username:password@tcp(host:port)/dbName
func NewDB(driverName, dataSourceName string) (*DB, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	return &DB{
		db:         db,
		driverName: driverName,
	}, nil
}

func (d *DB) DB() *sql.DB {
	return d.db
}

func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	return d.db.Exec(query, args...)
}

func (d *DB) MustExec(query string, args ...any) {
	_, err := d.db.Exec(query, args...)
	if err != nil {
		panic(err)
	}
}

func (d *DB) Begin() (*Tx, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}

	return &Tx{
		tx:         tx,
		driverName: d.driverName,
	}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) Table(nameOrRecord any) *Table {
	name, ok := nameOrRecord.(string)
	if !ok {
		name = getTableName(nameOrRecord)
	}

	return &Table{
		exe:        d.db,
		driverName: d.driverName,
		name:       name,
	}
}

func (d *DB) Insert(record any) error {
	return d.Table(getTableName(record)).Insert(record)
}

func (d *DB) BatchInsert(values any) error {
	l := reflect.ValueOf(values)
	if l.Kind() != reflect.Slice {
		return errors.New("not slice")
	}

	tx, err := d.Begin()
	for i := 0; i < l.Len(); i++ {
		err = tx.Insert(l.Index(i).Interface())
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) Update(record any) error {
	return d.Table(getTableName(record)).Update(record)
}

func (d *DB) BatchUpdate(values any) error {
	l := reflect.ValueOf(values)
	if l.Kind() != reflect.Slice {
		return errors.New("not slice")
	}

	tx, err := d.Begin()
	for i := 0; i < l.Len(); i++ {
		err = tx.Update(l.Index(i).Interface())
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) Save(record any) error {
	return d.Table(getTableName(record)).Save(record)
}

func (d *DB) MultiSave(values any) error {
	l := reflect.ValueOf(values)
	if l.Kind() != reflect.Slice {
		return errors.New("not slice")
	}

	tx, err := d.Begin()
	for i := 0; i < l.Len(); i++ {
		err = tx.Save(l.Index(i).Interface())
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) Select(records any, where string, args ...any) error {
	return d.Table(getTableNameBySlice(records)).Select(records, where, args...)
}

func (d *DB) SelectOne(record any, where string, args ...any) error {
	return d.Table(getTableName(record)).SelectOne(record, where, args...)
}
