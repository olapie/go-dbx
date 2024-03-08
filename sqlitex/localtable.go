package sqlitex

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.olapie.com/security"
	"go.olapie.com/times"
	"go.olapie.com/utils"
)

const (
	defaultLocalTableCacheSize = 1024
	minimumLocalTableCacheSize = 256
)

type LocalTableOptions[R any] struct {
	Clock             times.Clock
	MarshalFunc       func(r R) ([]byte, error)
	UnmarshalFunc     func(data []byte, r *R) error
	Password          string
	LocalCacheSize    int
	RemoteCacheSize   int
	DeletionCacheSize int
}

type LocalTable[R any] struct {
	db            *sql.DB
	localCache    *lru.Cache[string, R]
	remoteCache   *lru.Cache[string, R]
	deletionCache *lru.Cache[string, bool]
	options       LocalTableOptions[R]
}

func NewLocalTable[R any](db *sql.DB, optFns ...func(*LocalTableOptions[R])) *LocalTable[R] {
	t := &LocalTable[R]{
		db: db,
	}

	t.options.LocalCacheSize = defaultLocalTableCacheSize
	t.options.RemoteCacheSize = defaultLocalTableCacheSize
	t.options.DeletionCacheSize = defaultLocalTableCacheSize

	for _, fn := range optFns {
		fn(&t.options)
	}

	if t.options.Clock == nil {
		t.options.Clock = times.LocalClock{}
	}

	if t.options.LocalCacheSize < minimumLocalTableCacheSize {
		t.options.LocalCacheSize = minimumLocalTableCacheSize
	}
	if t.options.RemoteCacheSize < minimumLocalTableCacheSize {
		t.options.RemoteCacheSize = minimumLocalTableCacheSize
	}
	if t.options.DeletionCacheSize < minimumLocalTableCacheSize {
		t.options.DeletionCacheSize = minimumLocalTableCacheSize
	}

	t.localCache = MustGet(lru.New[string, R](t.options.LocalCacheSize))
	t.remoteCache = MustGet(lru.New[string, R](t.options.RemoteCacheSize))
	t.deletionCache = MustGet(lru.New[string, bool](t.options.DeletionCacheSize))

	// table remotes: localID, recordData, updateTime, synced
	// table locals: localID, recordData, createTime, updateTime
	// table deletions: localID, deleteTime

	MustGet(db.Exec(`CREATE TABLE IF NOT EXISTS remotes(
    id VARCHAR PRIMARY KEY,
    category INTEGER DEFAULT 0,
    data BLOB,
    update_time INTEGER,
    synced BOOL DEFAULT FALSE
)`))

	MustGet(db.Exec(`CREATE TABLE IF NOT EXISTS locals(
    id VARCHAR PRIMARY KEY,
    category INTEGER DEFAULT 0,
    data BLOB,
    create_time INTEGER,
    update_time INTEGER
)`))

	MustGet(db.Exec(`CREATE TABLE IF NOT EXISTS deletions(
    id VARCHAR PRIMARY KEY,
    category INTEGER DEFAULT 0,
    data BLOB,
    delete_time INTEGER
)`))

	return t
}

func (t *LocalTable[R]) SaveRemote(ctx context.Context, localID string, category int, record R, updateTime int64) error {
	// check delete_record, if it's deleted, then ignore
	// if updateTime < remotes.updateTime, then ignore
	// save: localID, recordData, updateTime(new), synced(true)
	exists, _ := t.deletionCache.Get(localID)
	if !exists {
		err := t.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT * FROM deletions WHERE id=?)`, localID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("query deletions: %w", err)
		}
	}

	if exists {
		log.Println("Skipped locally deleted record", localID)
		return nil
	}

	err := t.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT * FROM remotes WHERE id=? AND update_time>?)`,
		localID, updateTime).Scan(&exists)
	if err != nil {
		return fmt.Errorf("query remotes: %w", err)
	}

	if exists {
		log.Println("ignore outdated remote record", localID)
		return nil
	}

	data, err := t.encode(localID, record)
	if err != nil {
		return fmt.Errorf("encode: %s, %w", localID, err)
	}

	_, err = t.db.ExecContext(ctx, `REPLACE INTO remotes(id, category, data, update_time, synced) VALUES(?,?,?,?,1)`,
		localID, category, data, updateTime)
	if err != nil {
		return fmt.Errorf("replace into remotes: %s,%w", localID, err)
	}
	t.remoteCache.Add(localID, record)

	_, err = t.db.ExecContext(ctx, `DELETE FROM locals WHERE id=? AND update_time<=?`, localID, updateTime)
	if err != nil {
		return fmt.Errorf("delete locals: %s, %w", localID, err)
	}
	t.localCache.Remove(localID)

	return nil
}

func (t *LocalTable[R]) SaveLocal(ctx context.Context, localID string, category int, record R) error {
	// replace locals
	data, err := t.encode(localID, record)
	if err != nil {
		return fmt.Errorf("encode: %s, %w", localID, err)
	}

	_, err = t.db.ExecContext(ctx, `REPLACE INTO locals(id,category, data, update_time) VALUES(?,?,?,?)`,
		localID, category, data, t.options.Clock.Now().Unix())
	if err != nil {
		return fmt.Errorf("replace into locals: %s, %w", localID, err)
	}
	t.localCache.Add(localID, record)
	return nil
}

func (t *LocalTable[R]) Delete(ctx context.Context, localID string) error {
	// delete from locals
	// delete from remotes
	// save in delete_record
	_, err := t.db.ExecContext(ctx, `DELETE FROM locals WHERE id=?`, localID)
	if err != nil {
		return fmt.Errorf("delete locals: %s, %w", localID, err)
	}
	t.localCache.Remove(localID)

	var remoteData []byte
	var category int
	err = t.db.QueryRowContext(ctx, `SELECT category, data FROM remotes WHERE id=?`, localID).Scan(&category, &remoteData)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// don't need to keep deleted record as it doesn't exist remotely
		break
	case err != nil:
		return fmt.Errorf("query remotes: %s, %w", localID, err)
	default:
		_, err := t.db.ExecContext(ctx, `REPLACE INTO deletions(id, category, data, delete_time) VALUES (?,?,?,?)`,
			localID, category, remoteData, t.options.Clock.Now().Unix())
		if err != nil {
			return fmt.Errorf("replace into deletions: %s, %w", localID, err)
		} else {
			t.deletionCache.Add(localID, true)
		}

		_, err = t.db.ExecContext(ctx, `DELETE FROM remotes WHERE id=?`, localID)
		if err != nil {
			return fmt.Errorf("delete remotes: %s, %w", localID, err)
		}
		t.remoteCache.Remove(localID)
	}
	return nil
}

func (t *LocalTable[R]) Update(ctx context.Context, localID string, record R) error {
	data, err := t.encode(localID, record)
	if err != nil {
		return fmt.Errorf("encode record: %w", err)
	}

	isRemote, err := t.IsRemote(ctx, localID)
	if err != nil {
		return fmt.Errorf("is remote: %w", err)
	}

	if isRemote {
		return t.updateRemote(ctx, localID, record, &data)
	}

	isLocal, err := t.IsLocal(ctx, localID)
	if err != nil {
		return fmt.Errorf("is local: %w", err)
	}

	if isLocal {
		return t.updateLocal(ctx, localID, record, &data)
	}

	return sql.ErrNoRows
}

func (t *LocalTable[R]) UpdateLocal(ctx context.Context, localID string, record R) error {
	return t.updateLocal(ctx, localID, record, nil)
}

func (t *LocalTable[R]) UpdateRemote(ctx context.Context, localID string, record R) error {
	return t.updateRemote(ctx, localID, record, nil)
}

func (t *LocalTable[R]) IsRemote(ctx context.Context, localID string) (bool, error) {
	if t.remoteCache.Contains(localID) {
		return true, nil
	}
	var exists bool
	err := t.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT * FROM remotes WHERE id=?)`, localID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("query remotes: %w", err)
	}
	return exists, nil
}

func (t *LocalTable[R]) IsLocal(ctx context.Context, localID string) (bool, error) {
	if t.localCache.Contains(localID) {
		return true, nil
	}
	var exists bool
	err := t.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT * FROM locals WHERE id=?)`, localID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("query remotes: %w", err)
	}
	return exists, nil
}

func (t *LocalTable[R]) List(ctx context.Context, categories ...int) ([]R, error) {
	var where string
	if len(categories) > 0 {
		if len(categories) == 1 {
			where = fmt.Sprintf("category=%d", categories[0])
		} else {
			joinedCategories := strings.Join(toStringSlice(categories), ",")
			where = fmt.Sprintf("category in (%s)", joinedCategories)
		}
	}

	remoteIDs, remotes, err := t.list(ctx, "remotes", where)
	if err != nil {
		return nil, fmt.Errorf("list remotes: %w", err)
	}

	localIDs, locals, err := t.list(ctx, "locals", where)
	if err != nil {
		return nil, fmt.Errorf("list locals: %w", err)
	}

	ids := make(map[string]bool, len(remoteIDs)+len(localIDs))
	for _, id := range remoteIDs {
		ids[id] = true
	}

	l := remotes
	for i, v := range locals {
		if ids[localIDs[i]] {
			continue
		}
		l = append(l, v)
	}
	return l, nil
}

func (t *LocalTable[R]) ListExclusive(ctx context.Context, categories ...int) ([]R, error) {
	var where string
	if len(categories) > 0 {
		if len(categories) == 1 {
			where = fmt.Sprintf("category!=%d", categories[0])
		} else {
			joinedCategories := strings.Join(toStringSlice(categories), ",")
			where = fmt.Sprintf("category not in (%s)", joinedCategories)
		}
	}
	remoteIDs, remotes, err := t.list(ctx, "remotes", where)
	if err != nil {
		return nil, fmt.Errorf("list remotes: %w", err)
	}

	localIDs, locals, err := t.list(ctx, "locals", where)
	if err != nil {
		return nil, fmt.Errorf("list locals: %w", err)
	}

	ids := make(map[string]bool, len(remoteIDs)+len(localIDs))
	for _, id := range remoteIDs {
		ids[id] = true
	}

	l := remotes
	for i, v := range locals {
		if ids[localIDs[i]] {
			continue
		}
		l = append(l, v)
	}
	return l, nil
}

func (t *LocalTable[R]) ListRemotes(ctx context.Context) ([]R, error) {
	_, l, err := t.list(ctx, "remotes", "")
	return l, err
}

func (t *LocalTable[R]) ListUpdates(ctx context.Context) ([]R, error) {
	_, l, err := t.list(ctx, "remotes", "synced=0")
	return l, err
}

func (t *LocalTable[R]) ListLocals(ctx context.Context) ([]R, error) {
	_, l, err := t.list(ctx, "locals", "")
	return l, err
}

func (t *LocalTable[R]) ListDeletions(ctx context.Context) ([]R, error) {
	_, l, err := t.list(ctx, "deletions", "")
	return l, err
}

func (t *LocalTable[R]) BatchGetRemotes(ctx context.Context, localIDs ...string) ([]R, error) {
	if len(localIDs) == 0 {
		return nil, nil
	}
	condition, args := t.getCatchIDsCondition(localIDs...)
	_, l, err := t.list(ctx, "remotes", "id IN "+condition, args...)
	return l, err
}

func (t *LocalTable[R]) BatchGetLocals(ctx context.Context, localIDs ...string) ([]R, error) {
	if len(localIDs) == 0 {
		return nil, nil
	}
	condition, args := t.getCatchIDsCondition(localIDs...)
	_, l, err := t.list(ctx, "locals", "id IN "+condition, args...)
	return l, err
}

func (t *LocalTable[R]) RemoveDeletions(ctx context.Context, localIDs ...string) error {
	if len(localIDs) == 0 {
		return nil
	}
	condition, args := t.getCatchIDsCondition(localIDs...)
	_, err := t.db.ExecContext(ctx, `DELETE FROM deletions WHERE id IN `+condition, args...)
	if err != nil {
		return fmt.Errorf("remove deletionss: %w", err)
	}
	for _, id := range localIDs {
		t.localCache.Remove(id)
	}
	return nil
}

func (t *LocalTable[R]) RemoveAllRemotes(ctx context.Context) error {
	_, err := t.db.ExecContext(ctx, `DELETE FROM remotes`)
	return err
}

func (t *LocalTable[R]) Get(ctx context.Context, localID string) (record R, err error) {
	var ok bool
	record, ok = t.remoteCache.Get(localID)
	if ok {
		return record, nil
	}
	record, ok = t.localCache.Get(localID)
	if ok {
		return record, nil
	}
	var data []byte
	err = t.db.QueryRowContext(ctx, `SELECT data FROM remotes WHERE id=?`, localID).Scan(&data)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		break
	case err != nil:
		return record, fmt.Errorf("query remotes: %w", err)
	default:
		record, err = t.decode(localID, data)
		if err != nil {
			return record, err
		}
		t.remoteCache.Add(localID, record)
		return record, nil
	}

	err = t.db.QueryRowContext(ctx, `SELECT data FROM locals WHERE id=?`, localID).Scan(&data)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		break
	case err != nil:
		return record, fmt.Errorf("query locals: %w", err)
	default:
		record, err = t.decode(localID, data)
		if err != nil {
			return record, err
		}
		t.localCache.Add(localID, record)
		return record, nil
	}

	return record, sql.ErrNoRows
}

func (t *LocalTable[R]) CleanLocals(ctx context.Context) error {
	res, err := t.db.ExecContext(ctx, `DELETE FROM locals WHERE id IN (SELECT id FROM remotes)`)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		log.Println("failed getting affected rows count", err)
	} else {
		log.Printf("deleted %d locals which have been saved remotely\n", n)
	}
	return nil
}

func (t *LocalTable[R]) EncryptPlainData(ctx context.Context) error {
	tableNames := []string{"remotes", "locals", "deletions"}
	for _, name := range tableNames {
		m, err := t.encryptTable(ctx, name)
		if err != nil {
			return fmt.Errorf("encryptTable: %s, %w", name, err)
		}
		err = t.writeEncryptedData(ctx, name, m)
		if err != nil {
			return fmt.Errorf("writeEncryptedData: %s, %w", name, err)
		}
	}
	return nil
}

//
//func (t *LocalTable[R]) Migrate() error {
//	_, err := t.db.Exec(`REPLACE INTO remotes(id,data,update_time,synced)
//SELECT local_id,data,update_time,synced FROM remote_record`)
//	if err != nil {
//		return err
//	}
//
//	_, err = t.db.Exec(`REPLACE INTO locals(id,data,create_time,update_time)
//SELECT local_id,data,create_time,update_time FROM local_record`)
//	if err != nil {
//		return err
//	}
//
//	_, err = t.db.Exec(`REPLACE INTO deletions(id,data,delete_time)
//SELECT local_id,data,delete_time FROM deleted_record`)
//	if err != nil {
//		return err
//	}
//	return nil
//}

func (t *LocalTable[R]) updateLocal(ctx context.Context, localID string, record R, optionalData *[]byte) error {
	if optionalData == nil {
		data, err := t.encode(localID, record)
		if err != nil {
			return fmt.Errorf("encode record: %s, %w", localID, err)
		}
		optionalData = &data
	}
	_, err := t.db.ExecContext(ctx, `UPDATE locals SET data=?, update_time=? WHERE id=?`,
		*optionalData, t.options.Clock.Now().Unix(), localID)
	if err != nil {
		return fmt.Errorf("update locals: %w", err)
	}
	t.localCache.Add(localID, record)
	return nil
}

func (t *LocalTable[R]) updateRemote(ctx context.Context, localID string, record R, optionalData *[]byte) error {
	if optionalData == nil {
		data, err := t.encode(localID, record)
		if err != nil {
			return fmt.Errorf("encode: %s, %w", localID, err)
		}
		optionalData = &data
	}
	_, err := t.db.ExecContext(ctx, `UPDATE remotes SET data=?, update_time=?, synced=0 WHERE id=?`,
		*optionalData, t.options.Clock.Now().Unix(), localID)
	if err != nil {
		return fmt.Errorf("update remotes: %w", err)
	}
	t.remoteCache.Add(localID, record)
	return nil
}

func (t *LocalTable[R]) encryptTable(ctx context.Context, tableName string) (map[string][]byte, error) {
	rows, err := t.db.QueryContext(ctx, `SELECT id, data FROM `+tableName)
	if err != nil {
		return nil, fmt.Errorf("query locals: %w", err)
	}
	defer rows.Close()
	idToData := make(map[string][]byte)
	for rows.Next() {
		var localID string
		var data []byte
		err := rows.Scan(&localID, &data)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", tableName, err)
		}

		if security.IsEncrypted(data) {
			continue
		}

		data, err = security.Encrypt(data, t.options.Password+localID)
		if err != nil {
			return nil, fmt.Errorf("encrypt %s: %w", tableName, err)
		}
		idToData[localID] = data
	}
	return idToData, nil
}

func (t *LocalTable[R]) writeEncryptedData(ctx context.Context, tableName string, idToData map[string][]byte) error {
	query := fmt.Sprintf(`UPDATE %s SET data=? WHERE id=?`, tableName)
	for id, data := range idToData {
		_, err := t.db.ExecContext(ctx, query, data, id)
		if err != nil {
			return err
		}
		log.Println("Updated encrypted data", tableName, id)
	}
	return nil
}

func (t *LocalTable[R]) list(ctx context.Context, tableName string, where string, args ...any) ([]string, []R, error) {
	if where != "" {
		where = " where " + where
	}
	query := `SELECT id, data FROM ` + tableName + where
	rows, err := t.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("execute query: %s, %w", query, err)
	}
	defer rows.Close()
	return t.scan(rows, tableName)
}

func (t *LocalTable[R]) scan(rows *sql.Rows, tableName string) ([]string, []R, error) {
	var cache interface {
		Add(key string, value R) bool
		Get(key string) (R, bool)
	}

	if tableName == "locals" {
		cache = t.localCache
	} else if tableName == "remotes" {
		cache = t.remoteCache
	}

	var ids []string
	var records []R
	for rows.Next() {
		var localID string
		var data []byte

		err := rows.Scan(&localID, &data)
		if err != nil {
			return nil, nil, fmt.Errorf("scan %s: %w", tableName, err)
		}

		if cache != nil {
			if r, ok := cache.Get(localID); ok {
				ids = append(ids, localID)
				records = append(records, r)
				continue
			}
		}

		r, err := t.decode(localID, data)
		if err != nil {
			return nil, nil, fmt.Errorf("decode: %w", err)
		}

		if cache != nil {
			cache.Add(localID, r)
		}
		ids = append(ids, localID)
		records = append(records, r)
	}

	return ids, records, nil
}

func (t *LocalTable[R]) encode(localID string, r R) (data []byte, err error) {
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

	return security.Encrypt(data, t.options.Password+localID)
}

func (t *LocalTable[R]) decode(localID string, data []byte) (record R, err error) {
	if t.options.Password != "" {
		data, err = security.Decrypt(data, t.options.Password+localID)
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

func (t *LocalTable[R]) getCatchIDsCondition(ids ...string) (string, []any) {
	if len(ids) == 0 {
		return "", nil
	}
	var buf bytes.Buffer
	buf.WriteByte('(')
	for range ids {
		buf.WriteString("?,")
	}
	buf.Truncate(len(ids) * 2)
	buf.WriteByte(')')
	args := toAnySlice(ids)
	return buf.String(), args
}

func toAnySlice[T any](a []T) []any {
	l := make([]any, len(a))
	for i, v := range a {
		l[i] = v
	}
	return l
}

func toStringSlice[T any](a []T) []string {
	l := make([]string, len(a))
	for i, v := range a {
		l[i] = fmt.Sprint(v)
	}
	return l
}
