package sqlite

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"code.olapie.com/conv"
	"code.olapie.com/log"
	sqlx "code.olapie.com/sqlx"
	"github.com/golang/protobuf/proto"
)

type Clock interface {
	Now() time.Time
}

type KVStore struct {
	ID       any
	clock    Clock
	db       *sql.DB
	mu       sync.RWMutex
	filename string
}

func NewKVStore(filename string, clock Clock) *KVStore {
	db := Open(filename)
	r := &KVStore{
		clock:    clock,
		db:       db,
		filename: filename,
	}

	if r.clock == nil {
		r.clock = localClock{}
	}

	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS kv(
k VARCHAR(255) PRIMARY KEY, 
v BLOB NOT NULL,
updated_at BIGINT NOT NULL
)`)
	if err != nil {
		log.G().Fatal("cannot create table", log.Error(err))
	}
	return r
}

func (s *KVStore) Filename() string {
	return s.filename
}

func (s *KVStore) SaveInt64(key string, val int64) {
	s.mu.Lock()
	_, err := s.db.Exec("REPLACE INTO kv(k,v,updated_at) VALUES(?1,?2,?3)",
		key, fmt.Sprint(val), s.clock.Now())
	s.mu.Unlock()
	if err != nil {
		log.G().Error("exec", log.String("key", key), log.Int64("value", val), log.Error(err))
	}
}

func (s *KVStore) GetInt64(key string) (int64, error) {
	var v string
	s.mu.RLock()
	err := s.db.QueryRow("SELECT v FROM kv WHERE k=?", key).Scan(&v)
	s.mu.RUnlock()
	if err != nil {
		return 0, err
	}

	n, err := conv.ToInt64(v)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *KVStore) SaveData(key string, data []byte) {
	s.mu.Lock()
	_, err := s.db.Exec("REPLACE INTO kv(k,v,updated_at) VALUES(?1,?2,?3)", key, data, s.clock.Now())
	s.mu.Unlock()
	if err != nil {
		log.G().Error("exec", log.String("key", key), log.Error(err))
	}
}

func (s *KVStore) GetData(key string) ([]byte, error) {
	var v []byte
	s.mu.RLock()
	err := s.db.QueryRow("SELECT v FROM kv WHERE k=?", key).Scan(&v)
	s.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return v, nil
}

func (s *KVStore) SaveString(key string, str string) {
	s.SaveData(key, []byte(str))
}

func (s *KVStore) GetString(key string) (string, error) {
	data, err := s.GetData(key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *KVStore) SavePB(key string, msg proto.Message) {
	data, err := proto.Marshal(msg)
	if err != nil {
		log.G().Error("marshal", log.String("key", key), log.Error(err))
		return
	}
	s.mu.Lock()
	_, err = s.db.Exec("REPLACE INTO kv(k,v,updated_at) VALUES(?1,?2,?3)", key, data, s.clock.Now())
	s.mu.Unlock()
	if err != nil {
		log.G().Error("exec", log.String("key", key), log.Error(err))
	}
}

func (s *KVStore) GetPB(key string, msg proto.Message) error {
	var v []byte
	s.mu.RLock()
	err := s.db.QueryRow("SELECT v FROM kv WHERE k=?", key).Scan(&v)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	return proto.Unmarshal(v, msg)
}

func (s *KVStore) SaveJSON(key string, obj any) {
	v := sqlx.JSON(obj)
	s.mu.Lock()
	var err error
	if v == nil {
		_, err = s.db.Exec("DELETE FROM kv WHERE k=?1", key)
	} else {
		_, err = s.db.Exec("REPLACE INTO kv(k,v,updated_at) VALUES(?1,?2,?3)", key, v, s.clock.Now())
	}
	s.mu.Unlock()
	if err != nil {
		log.G().Error("exec", log.String("key", key), log.Error(err))
	}
}

func (s *KVStore) GetJSON(key string, ptrToObj any) error {
	s.mu.RLock()
	err := s.db.QueryRow("SELECT v FROM kv WHERE k=?", key).Scan(sqlx.JSON(ptrToObj))
	s.mu.RUnlock()
	return err
}

func (s *KVStore) Close() error {
	s.mu.Lock()
	err := s.db.Close()
	s.mu.Unlock()
	return err
}

type localClock struct {
}

func (l localClock) Now() time.Time {
	return time.Now()
}
