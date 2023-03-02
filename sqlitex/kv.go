package sqlitex

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"go.olapie.com/times"
	"go.olapie.com/utils"
)

type KVTableOptions struct {
	Clock times.Clock
}

type KVTable struct {
	options KVTableOptions
	db      *sql.DB
	mu      sync.RWMutex
	name    string
}

func NewKVTable(db *sql.DB, optFns ...func(options *KVTableOptions)) *KVTable {
	r := &KVTable{
		db: db,
	}

	for _, fn := range optFns {
		fn(&r.options)
	}

	if r.options.Clock == nil {
		r.options.Clock = times.LocalClock{}
	}

	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS kv(
k VARCHAR(255) PRIMARY KEY, 
v BLOB NOT NULL,
updated_at BIGINT NOT NULL
)`)
	if err != nil {
		panic(err)
	}
	return r
}

func (t *KVTable) SaveInt64(key string, val int64) error {
	t.mu.Lock()
	_, err := t.db.Exec("REPLACE INTO kv(k,v,updated_at) VALUES(?1,?2,?3)",
		key, fmt.Sprint(val), t.options.Clock.Now())
	t.mu.Unlock()
	return err
}

func (t *KVTable) Int64(key string) (int64, error) {
	var v string
	t.mu.RLock()
	err := t.db.QueryRow("SELECT v FROM kv WHERE k=?", key).Scan(&v)
	t.mu.RUnlock()
	if err != nil {
		return 0, err
	}

	n, err := utils.ToInt64(v)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (t *KVTable) SaveString(key string, str string) error {
	return t.SaveBytes(key, []byte(str))
}

func (t *KVTable) String(key string) (string, error) {
	data, err := t.Bytes(key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (t *KVTable) SaveBytes(key string, data []byte) error {
	t.mu.Lock()
	_, err := t.db.Exec("REPLACE INTO kv(k,v,updated_at) VALUES(?1,?2,?3)", key, data, t.options.Clock.Now())
	t.mu.Unlock()
	return err
}

func (t *KVTable) Bytes(key string) ([]byte, error) {
	var v []byte
	t.mu.RLock()
	err := t.db.QueryRow("SELECT v FROM kv WHERE k=?", key).Scan(&v)
	t.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return v, nil
}

func (t *KVTable) SaveObject(key string, obj any) error {
	data, err := t.encode(obj)
	if err != nil {
		return err
	}
	t.mu.Lock()
	if obj == nil {
		_, err = t.db.Exec("DELETE FROM kv WHERE k=?1", key)
	} else {
		_, err = t.db.Exec("REPLACE INTO kv(k,v,updated_at) VALUES(?1,?2,?3)", key, data, t.options.Clock.Now())
	}
	t.mu.Unlock()
	return err
}

func (t *KVTable) GetObject(key string, ptrToObj any) error {
	var data []byte
	t.mu.RLock()
	err := t.db.QueryRow("SELECT v FROM kv WHERE k=?", key).Scan(&data)
	t.mu.RUnlock()
	if err != nil {
		return err
	}
	return t.decode(data, ptrToObj)
}

func (t *KVTable) ListKeys(prefix string) ([]string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	query := "SELECT k FROM kv"
	if prefix != "" {
		query = "SELECT k FROM kv WHERE k LIKE '" + prefix + "%'"
	}
	rows, err := t.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed executing %s: %w", query, err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		err = rows.Scan(&key)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (t *KVTable) Delete(key string) error {
	t.mu.Lock()
	_, err := t.db.Exec("DELETE FROM kv WHERE k=?", key)
	t.mu.Unlock()
	return err
}

func (t *KVTable) DeleteWithPrefix(prefix string) error {
	t.mu.Lock()
	_, err := t.db.Exec("DELETE FROM kv WHERE k like '" + prefix + "%'")
	t.mu.Unlock()
	return err
}

func (t *KVTable) Exists(key string) (bool, error) {
	t.mu.RLock()
	var exists bool
	err := t.db.QueryRow("SELECT EXISTS(SELECT * FROM kv WHERE k=?)", key).Scan(&exists)
	t.mu.RUnlock()
	return exists, err
}

func (t *KVTable) Close() error {
	if t.db == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}

func (t *KVTable) encode(obj any) ([]byte, error) {
	data, err := utils.Marshal(obj)
	if err != nil {
		return json.Marshal(obj)
	}
	return data, nil
}

func (t *KVTable) decode(data []byte, ptrToObj any) error {
	err := utils.Unmarshal(data, ptrToObj)
	if err != nil {
		err = json.Unmarshal(data, ptrToObj)
	}
	return err
}
