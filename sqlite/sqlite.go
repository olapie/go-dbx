package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func Open(fileName string) *sql.DB {
	err := os.MkdirAll(filepath.Dir(fileName), 0755)
	if err != nil {
		log.Panicf("Mkdir: %v", err)
	}

	_, err = os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Panicf("Open file: %v", err)
	}

	dataSource := fmt.Sprintf("file:%s?cache=shared", fileName)
	db, err := sql.Open("sqlite3", dataSource)
	if err != nil {
		log.Panicf("Open db: %v", err)
	}
	return db
}
