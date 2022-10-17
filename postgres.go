package sqlx

import (
	"database/sql"
	"fmt"
	"os/user"

	"code.olapie.com/log"
)

func GetPostgresDSN(name, host string, port int, user, password string, sslEnabled bool) string {
	if host == "" {
		host = "localhost"
	}

	if port == 0 {
		port = 5432
	}

	url := fmt.Sprintf("%s:%d/%s", host, port, name)
	if user == "" {
		url = "postgres://" + url
	} else {
		if password == "" {
			url = "postgres://" + user + "@" + url
		} else {
			url = "postgres://" + user + ":" + password + "@" + url
		}
	}
	if !sslEnabled {
		url = url + "?sslmode=disable"
	}
	return url
}

func OpenPostgres(dbURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}

func MustOpenPostgres(dbURL string) *sql.DB {
	db, err := OpenPostgres(dbURL)
	if err != nil {
		log.G().Panic("cannot open", log.String("url", dbURL), log.Error(err))
	}
	return db
}

func LocalPostgresDSN(unixSocket bool) string {
	u, err := user.Current()
	if err != nil {
		log.G().Error("cannot get current user", log.Error(err))
		return ""
	}
	if unixSocket {
		return fmt.Sprintf("postgres:///%s?host=/var/run/postgresql/", u.Username)
	}
	return GetPostgresDSN(u.Username, "localhost", 5432, u.Username, "", false)
}

func OpenLocalPostgres() (*sql.DB, error) {
	url := LocalPostgresDSN(false)
	if db, err := OpenPostgres(LocalPostgresDSN(false)); err == nil {
		log.G().Debug("Connected via unix socket")
		return db, nil
	}
	url = LocalPostgresDSN(true)
	db, err := OpenPostgres(url)
	if err == nil {
		log.G().Debug("Connected via tcp socket")
	}
	return db, err
}

func MustOpenLocalPostgres() *sql.DB {
	db, err := OpenLocalPostgres()
	if err != nil {
		log.G().Panic("cannot open local db", log.Error(err))
	}
	return db
}
