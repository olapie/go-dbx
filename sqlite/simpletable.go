package sqlite

import (
	"database/sql"
	"fmt"
	"reflect"
	"sync"
	"time"

	"code.olapie.com/sqlx"
)

type PrimaryKey[T IntOrString] interface {
	PrimaryKey() T
}

type IntOrString interface {
	int | int32 | int64 | string
}

type SimpleTable[K IntOrString, M PrimaryKey[K]] struct {
	name     string
	db       *sql.DB
	mu       sync.RWMutex
	newModel func() M
	stmts    struct {
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
	Now Clock
}

func NewSimpleTable[K IntOrString, M PrimaryKey[K]](db *sql.DB, name string, newModel func() M) (*SimpleTable[K, M], error) {
	var zero K
	var typ string
	if reflect.ValueOf(zero).Kind() == reflect.String {
		typ = "VARCHAR(64)"
	} else {
		typ = "BIGINT"
	}

	_, err := db.Exec(fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s(
id %s PRIMARY KEY,
data BLOB,
updated_at BIGINT
)`, name, typ))
	if err != nil {
		return nil, err
	}

	t := &SimpleTable[K, M]{
		name:     name,
		db:       db,
		newModel: newModel,
	}

	t.stmts.insert = sqlx.MustPrepare(db, `INSERT INTO %s(id,data,updated_at) VALUES(?,?,?)`, name)
	t.stmts.update = sqlx.MustPrepare(db, `UPDATE %s SET data=?,updated_at=? WHERE id=?`, name)
	t.stmts.save = sqlx.MustPrepare(db, `REPLACE INTO %s(id,data,updated_at) VALUES(?,?,?)`, name)
	t.stmts.get = sqlx.MustPrepare(db, `SELECT data FROM %s WHERE id=?`, name)
	t.stmts.listAll = sqlx.MustPrepare(db, `SELECT data FROM %s`, name)
	t.stmts.listGreaterThan = sqlx.MustPrepare(db, `SELECT data FROM %s WHERE id>? ORDER BY id ASC LIMIT ?`, name)
	t.stmts.listLessThan = sqlx.MustPrepare(db, `SELECT data FROM %s WHERE id<? ORDER BY id DESC LIMIT ?`, name)
	t.stmts.delete = sqlx.MustPrepare(db, `DELETE FROM %s WHERE id=?`, name)
	t.stmts.deleteGreaterThan = sqlx.MustPrepare(db, `DELETE FROM %s WHERE id>?`, name)
	t.stmts.deleteLessThan = sqlx.MustPrepare(db, `DELETE FROM %s WHERE id<?`, name)
	return t, nil
}

func (t *SimpleTable[K, M]) now() int64 {
	if t.Now != nil {
		return t.Now.Now().Unix()
	}
	return time.Now().Unix()
}

func (t *SimpleTable[K, M]) Insert(v PrimaryKey[K]) error {
	t.mu.Lock()
	_, err := t.stmts.insert.Exec(v.PrimaryKey(), sqlx.JSON(v), t.now())
	t.mu.Unlock()
	return err
}

func (t *SimpleTable[K, M]) Update(v PrimaryKey[K]) error {
	t.mu.Lock()
	_, err := t.stmts.update.Exec(v.PrimaryKey(), sqlx.JSON(v), t.now())
	t.mu.Unlock()
	return err
}

func (t *SimpleTable[K, M]) Save(v PrimaryKey[K]) error {
	t.mu.Lock()
	_, err := t.stmts.save.Exec(v.PrimaryKey(), sqlx.JSON(v), t.now())
	t.mu.Unlock()
	return err
}

func (t *SimpleTable[K, M]) Get(key K) (M, error) {
	t.mu.RLock()
	var zero M
	m := t.newModel()
	err := t.stmts.get.QueryRow(key).Scan(sqlx.JSON(m))
	t.mu.RUnlock()
	if err != nil {
		return zero, err
	}
	return m, err
}

func (t *SimpleTable[K, M]) ListAll() ([]M, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	rows, err := t.stmts.listAll.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return t.readList(rows)
}

func (t *SimpleTable[K, M]) ListGreaterThan(key K, limit int) ([]M, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	rows, err := t.stmts.listGreaterThan.Query(key, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return t.readList(rows)
}

func (t *SimpleTable[K, M]) ListLessThan(key K, limit int) ([]M, error) {
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

func (t *SimpleTable[K, M]) readList(rows *sql.Rows) ([]M, error) {
	var l []M
	for rows.Next() {
		var v M
		err := rows.Scan(sqlx.JSON(&v))
		if err != nil {
			return nil, err
		}
		l = append(l, v)
	}
	return l, nil
}

func (t *SimpleTable[K, M]) Delete(key K) error {
	_, err := t.stmts.delete.Exec(key)
	return err
}

func (t *SimpleTable[K, M]) DeleteGreaterThan(key K) error {
	_, err := t.stmts.deleteGreaterThan.Exec(key)
	return err
}

func (t *SimpleTable[K, M]) DeleteLessThan(key K) error {
	_, err := t.stmts.deleteLessThan.Exec(key)
	return err
}
