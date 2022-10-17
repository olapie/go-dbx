package sqlx

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"text/template"

	"code.olapie.com/log"
)

var Debug = false

const (
	SQLITE   = "sqlite"
	MYSQL    = "mysql"
	POSTGRES = "postgres"
)

type ColumnScanner interface {
	Scan(dest ...any) error
}

type Executor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func MustPrepare(db *sql.DB, format string, args ...any) *sql.Stmt {
	query := fmt.Sprintf(format, args...)
	stmt, err := db.Prepare(query)
	if err != nil {
		log.G().Panic("cannot prepare statement", log.String("query", query), log.Error(err))
	}
	return stmt
}

type readDirAndFileFS interface {
	fs.ReadDirFS
	fs.ReadFileFS
}

func ExecSQLDir(db *sql.DB, target fs.FS, params map[string]any) error {
	return fs.WalkDir(target, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Ext(d.Name()) != ".sql" {
			return nil
		}

		content, err := fs.ReadFile(target, path)
		if err != nil {
			return fmt.Errorf("read file %s: %w", path, err)
		}

		if len(params) != 0 {
			tpl, err := template.ParseGlob(string(content))
			if err != nil {
				return fmt.Errorf("parse template %s: %w", path, err)
			}

			var buf bytes.Buffer
			err = tpl.Execute(&buf, params)
			if err != nil {
				return fmt.Errorf("execute template %s: %w", path, err)
			}
			content = buf.Bytes()
		}

		_, err = db.Exec(string(content))
		if err != nil {
			return fmt.Errorf("execute %s: %w", path, err)
		}
		return nil
	})
}
