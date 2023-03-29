package sqlitex

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"go.olapie.com/dbx/sqlutil"
	"go.olapie.com/security"
	"go.olapie.com/times"
	"go.olapie.com/utils"
)

type SimpleTableOptions[K SimpleKey, R any] struct {
	Clock         times.Clock
	MarshalFunc   func(r R) ([]byte, error)
	UnmarshalFunc func(data []byte, r *R) error
	Password      string
}

type SimpleKey interface {
	int | int32 | int64 | string
}

type SimpleTable[K SimpleKey, R any] struct {
	options SimpleTableOptions[K, R]
	name    string
	db      *sql.DB
	mu      sync.RWMutex
	stmts   struct {
		insert            *sql.Stmt
		update            *sql.Stmt
		save              *sql.Stmt
		get               *sql.Stmt
		listAll           *sql.Stmt
		listGreaterThan   *sql.Stmt
		listLessThan      *sql.Stmt
		delete            *sql.Stmt
		deleteGreaterThan *sql.Stmt
		deleteLessThan    *sql.Stmt
	}
	pkFn func(r R) K
}

func NewSimpleTable[K SimpleKey, R any](db *sql.DB, name string, primaryKeyFunc func(r R) K, optFns ...func(options *SimpleTableOptions[K, R])) (*SimpleTable[K, R], error) {
	var zero K
	var typ string
	if reflect.ValueOf(zero).Kind() == reflect.String {
		typ = "VARCHAR(64)"
	} else {
		typ = "BIGINT"
	}

	query := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s(
id %s PRIMARY KEY,
data BLOB,
updated_at BIGINT
)`, name, typ)
	_, err := db.Exec(query)
	if err != nil {
		return nil, err
	}

	t := &SimpleTable[K, R]{
		name: name,
		db:   db,
		pkFn: primaryKeyFunc,
	}

	for _, fn := range optFns {
		fn(&t.options)
	}

	if t.options.Clock == nil {
		t.options.Clock = times.LocalClock{}
	}

	t.stmts.insert = sqlutil.MustPrepare(db, `INSERT INTO %s(id,data,updated_at) VALUES(?,?,?)`, name)
	t.stmts.update = sqlutil.MustPrepare(db, `UPDATE %s SET data=?,updated_at=? WHERE id=?`, name)
	t.stmts.save = sqlutil.MustPrepare(db, `REPLACE INTO %s(id,data,updated_at) VALUES(?,?,?)`, name)
	t.stmts.get = sqlutil.MustPrepare(db, `SELECT data FROM %s WHERE id=?`, name)
	t.stmts.listAll = sqlutil.MustPrepare(db, `SELECT id,data FROM %s ORDER BY updated_at`, name)
	t.stmts.listGreaterThan = sqlutil.MustPrepare(db, `SELECT id,data FROM %s WHERE id>? ORDER BY id ASC LIMIT ?`, name)
	t.stmts.listLessThan = sqlutil.MustPrepare(db, `SELECT id,data FROM %s WHERE id<? ORDER BY id DESC LIMIT ?`, name)
	t.stmts.delete = sqlutil.MustPrepare(db, `DELETE FROM %s WHERE id=?`, name)
	t.stmts.deleteGreaterThan = sqlutil.MustPrepare(db, `DELETE FROM %s WHERE id>?`, name)
	t.stmts.deleteLessThan = sqlutil.MustPrepare(db, `DELETE FROM %s WHERE id<?`, name)
	return t, nil
}

func (t *SimpleTable[K, R]) Insert(v R) error {
	key := t.pkFn(v)
	b, err := t.encode(key, v)
	if err != nil {
		return err
	}
	t.mu.Lock()
	_, err = t.stmts.insert.Exec(key, b, t.options.Clock.Now())
	t.mu.Unlock()
	return err
}

func (t *SimpleTable[K, R]) Update(v R) error {
	key := t.pkFn(v)
	b, err := t.encode(key, v)
	if err != nil {
		return err
	}
	t.mu.Lock()
	_, err = t.stmts.update.Exec(key, b, t.options.Clock.Now())
	t.mu.Unlock()
	return err
}

func (t *SimpleTable[K, R]) Save(v R) error {
	key := t.pkFn(v)
	b, err := t.encode(key, v)
	if err != nil {
		return err
	}
	t.mu.Lock()
	_, err = t.stmts.save.Exec(key, b, t.options.Clock.Now())
	t.mu.Unlock()
	return err
}

func (t *SimpleTable[K, R]) Get(key K) (R, error) {
	var data []byte
	t.mu.RLock()
	err := t.stmts.get.QueryRow(key).Scan(&data)
	t.mu.RUnlock()
	if err != nil {
		var zero R
		return zero, err
	}
	return t.decode(key, data)
}

func (t *SimpleTable[K, R]) ListAll() ([]R, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	rows, err := t.stmts.listAll.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return t.readList(rows)
}

func (t *SimpleTable[K, R]) ListGreaterThan(key K, limit int) ([]R, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	rows, err := t.stmts.listGreaterThan.Query(key, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return t.readList(rows)
}

func (t *SimpleTable[K, R]) ListLessThan(key K, limit int) ([]R, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	rows, err := t.stmts.listLessThan.Query(key, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	l, err := t.readList(rows)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
	return l, nil
}

func (t *SimpleTable[K, R]) readList(rows *sql.Rows) ([]R, error) {
	var l []R
	var data []byte
	var key K
	for rows.Next() {
		err := rows.Scan(&key, &data)
		if err != nil {
			return nil, err
		}
		v, err := t.decode(key, data)
		if err != nil {
			return nil, err
		}
		l = append(l, v)
	}
	return l, nil
}

func (t *SimpleTable[K, R]) Delete(key K) error {
	t.mu.Lock()
	_, err := t.stmts.delete.Exec(key)
	t.mu.Unlock()
	return err
}

func (t *SimpleTable[K, R]) DeleteGreaterThan(key K) error {
	t.mu.Lock()
	_, err := t.stmts.deleteGreaterThan.Exec(key)
	t.mu.Unlock()
	return err
}

func (t *SimpleTable[K, R]) DeleteLessThan(key K) error {
	t.mu.Lock()
	_, err := t.stmts.deleteLessThan.Exec(key)
	t.mu.Unlock()
	return err
}

func (t *SimpleTable[K, R]) encode(key K, r R) (data []byte, err error) {
	if t.options.MarshalFunc != nil {
		data, err = t.options.MarshalFunc(r)
	} else {
		data, err = utils.Marshal(r)
		if err != nil {
			data, err = json.Marshal(r)
		}
	}

	if err != nil {
		return
	}

	if t.options.Password == "" {
		return
	}

	return security.Encrypt(data, t.options.Password+fmt.Sprint(key))
}

func (t *SimpleTable[K, R]) decode(key K, data []byte) (record R, err error) {
	if t.options.Password != "" {
		data, err = security.Decrypt(data, t.options.Password+fmt.Sprint(key))
		if err != nil {
			return
		}
	}

	if t.options.UnmarshalFunc != nil {
		err := t.options.UnmarshalFunc(data, &record)
		return record, err
	}

	err = utils.Unmarshal(data, &record)
	if err != nil {
		err = json.Unmarshal(data, &record)
	}
	return
}
