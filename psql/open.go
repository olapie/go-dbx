package psql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os/user"
	"sync"

	"code.olapie.com/sugar/v2"
)

type OpenOptions struct {
	UnixSocket bool
	Host       string
	Port       int
	User       string
	Password   string
	Database   string
	Schema     string
	SSL        bool
}

func NewOpenOptions() *OpenOptions {
	return &OpenOptions{
		Host: "localhost",
		Port: 5432,
	}
}

func (c *OpenOptions) String() string {
	if c.UnixSocket {
		u, err := user.Current()
		if err != nil {
			log.Println(err)
			return ""
		}
		if c.Schema == "" {
			return fmt.Sprintf("postgres:///%s?host=/var/run/postgresql/", u.Username)
		} else {
			return fmt.Sprintf("postgres:///%s?host=/var/run/postgresql/&search_path=%s", u.Username, c.Schema)
		}
	}
	host := c.Host
	port := c.Port
	if host == "" {
		host = "localhost"
	}

	if port == 0 {
		port = 5432
	}

	connStr := fmt.Sprintf("%s:%d", host, port)
	if c.Database != "" {
		connStr += "/" + c.Database
	}
	if c.User == "" {
		connStr = "postgres://" + connStr
	} else {
		if c.Password == "" {
			connStr = "postgres://" + c.User + "@" + connStr
		} else {
			connStr = "postgres://" + c.User + ":" + c.Password + "@" + connStr
		}
	}
	query := url.Values{}
	if !c.SSL {
		query.Add("sslmode", "disable")
	}
	if c.Schema != "" {
		query.Add("search_path", c.Schema)
	}
	if len(query) == 0 {
		return connStr
	}
	return connStr + "?" + query.Encode()
}

func Open(options *OpenOptions) (*sql.DB, error) {
	if options == nil {
		options = NewOpenOptions()
	}
	connString := options.String()
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("open: %s, %w", connString, err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("ping: %s, %w", connString, err)
	}
	return db, nil
}

func MustOpen(options *OpenOptions) *sql.DB {
	return sugar.MustGet(Open(options))
}

func OpenLocal() (*sql.DB, error) {
	if db, err := Open(&OpenOptions{UnixSocket: true}); err == nil {
		log.Println("Connected via unix socket")
		return db, nil
	}
	db, err := Open(NewOpenOptions())
	if err == nil {
		log.Println("Connected via tcp socket")
	}
	return db, err
}

func MustOpenLocal() *sql.DB {
	return sugar.MustGet(OpenLocal())
}

type Factory[T any] interface {
	Get(ctx context.Context, repoID string) T
}

type NewRepoFunc[T any] func(ctx context.Context, db *sql.DB) T

type factoryImpl[T any] struct {
	mu      sync.RWMutex
	cache   map[string]T
	options *OpenOptions
	fn      NewRepoFunc[T]
}

func NewFactory[T any](options *OpenOptions, fn NewRepoFunc[T]) Factory[T] {
	f := &factoryImpl[T]{
		options: options,
		cache:   make(map[string]T),
		fn:      fn,
	}
	return f
}

func (f *factoryImpl[T]) Get(ctx context.Context, repoID string) T {
	f.mu.RLock()
	r, ok := f.cache[repoID]
	f.mu.RUnlock()
	if ok {
		return r
	}

	f.mu.Lock()
	r, ok = f.cache[repoID]
	if !ok {
		opt := *f.options
		opt.Schema = repoID
		db := MustOpen(&opt)
		r = f.fn(ctx, db)
		f.cache[repoID] = r
	}
	f.mu.Unlock()
	return r
}
