package psql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os/user"
	"sync"

	"go.olapie.com/logs"
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

func Open(ctx context.Context, options *OpenOptions) (*sql.DB, error) {
	if options == nil {
		options = NewOpenOptions()
	}
	connString := options.String()
	logger := logs.FromCtx(ctx)
	logger.Info("opening postgres",
		slog.String("user", options.User),
		slog.String("database", options.Database),
		slog.String("schema", options.Schema),
		slog.Bool("unix_socket", options.UnixSocket))
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("open: %s, %w", connString, err)
	}

	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ping: %s, %w", connString, err)
	}
	logger.Info("opening postgres",
		slog.String("user", options.User),
		slog.String("database", options.Database),
		slog.String("schema", options.Schema),
		slog.Bool("unix_socket", options.UnixSocket))
	return db, nil
}

func MustOpen(ctx context.Context, options *OpenOptions) *sql.DB {
	return MustGet(Open(ctx, options))
}

func OpenLocal(ctx context.Context) (*sql.DB, error) {
	if db, err := Open(ctx, &OpenOptions{UnixSocket: true}); err == nil {
		return db, nil
	}
	return Open(ctx, NewOpenOptions())
}

func MustOpenLocal(ctx context.Context) *sql.DB {
	return MustGet(OpenLocal(ctx))
}

var connStringToDBCache sync.Map

type RepoFactory[T any] interface {
	Get(ctx context.Context, schema string) T
}

type NewRepoFunc[T any] func(ctx context.Context, db *sql.DB) T

type repoFactoryImpl[T any] struct {
	mu      sync.RWMutex
	cache   map[string]T
	options *OpenOptions
	fn      NewRepoFunc[T]
}

func NewRepoFactory[T any](options *OpenOptions, fn NewRepoFunc[T]) RepoFactory[T] {
	f := &repoFactoryImpl[T]{
		options: options,
		cache:   make(map[string]T),
		fn:      fn,
	}
	return f
}

func (f *repoFactoryImpl[T]) Get(ctx context.Context, schema string) T {
	f.mu.RLock()
	repo, ok := f.cache[schema]
	f.mu.RUnlock()
	if ok {
		return repo
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	repo, ok = f.cache[schema]
	if ok {
		return repo
	}

	opt := *f.options
	opt.Schema = schema
	connStr := opt.String()
	if dbVal, ok := connStringToDBCache.Load(connStr); ok {
		repo = f.fn(ctx, dbVal.(*sql.DB))
	} else {
		db := MustOpen(ctx, &opt)
		connStringToDBCache.Store(connStr, db)
		repo = f.fn(ctx, db)
	}
	f.cache[schema] = repo
	return repo
}

// MustGet eliminates nil err and panics if err isn't nil
func MustGet[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
