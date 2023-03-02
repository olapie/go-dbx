package sqlitex

import (
	"database/sql"
	"errors"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"go.olapie.com/utils"
	"math/rand"
	"strings"
	"testing"
)

func createTable[K SimpleKey, R SimpleTableRecord[K]](t *testing.T) *SimpleTable[K, R] {
	t.Log("createTable")

	db, err := sql.Open("sqlite3", "file::memory:")
	if err != nil {
		t.Fatal(err)
	}

	name := "test" + strings.ReplaceAll(uuid.NewString(), "-", "")
	tbl, err := NewSimpleTable[K, R](db, name)
	utils.MustNotErrorT(t, err)
	return tbl
}

type IntItem struct {
	ID    int64
	Name  string
	Score float64
}

func (i *IntItem) PrimaryKey() int64 {
	return i.ID
}

func newIntItem() *IntItem {
	return &IntItem{
		ID:    rand.Int63(),
		Name:  uuid.NewString(),
		Score: float64(rand.Int63()) / float64(3),
	}
}

type StringItem struct {
	ID    string
	Name  string
	Score float64
}

func (i *StringItem) PrimaryKey() string {
	return i.ID
}

func newStringItem() *StringItem {
	return &StringItem{
		ID:    uuid.NewString(),
		Name:  uuid.NewString(),
		Score: float64(rand.Int63()) / float64(3),
	}
}

func TestIntTable(t *testing.T) {
	t.Log("TestIntTable")
	tbl := createTable[int64, *IntItem](t)
	var items []*IntItem
	item := newIntItem()
	items = append(items, item)
	err := tbl.Insert(item)
	utils.MustNotErrorT(t, err)
	v, err := tbl.Get(item.ID)
	utils.MustNotErrorT(t, err)
	utils.MustEqualT(t, item, v)

	item = newIntItem()
	item.ID = items[0].ID + 1
	err = tbl.Insert(item)
	utils.MustNotErrorT(t, err)
	items = append(items, item)

	l, err := tbl.ListAll()
	utils.MustNotErrorT(t, err)
	utils.MustTrueT(t, len(l) != 0)
	utils.MustEqualT(t, items, l)

	l, err = tbl.ListGreaterThan(item.ID, 10)
	utils.MustNotErrorT(t, err)
	utils.MustTrueT(t, len(l) == 0)

	l, err = tbl.ListLessThan(item.ID+1, 10)
	utils.MustNotErrorT(t, err)
	utils.MustEqualT(t, 2, len(l))
	//t.Log(l[0].ID, l[1].ID)
	utils.MustTrueT(t, l[0].ID < l[1].ID)

	err = tbl.Delete(item.ID)
	utils.MustNotErrorT(t, err)

	v, err = tbl.Get(item.ID)
	utils.MustErrorT(t, err)
	utils.MustEqualT(t, true, errors.Is(err, sql.ErrNoRows))
}

func TestStringTable(t *testing.T) {
	t.Log("TestStringTable")

	tbl := createTable[string, *StringItem](t)
	var items []*StringItem
	item := newStringItem()
	t.Log(item.PrimaryKey())
	items = append(items, item)
	err := tbl.Insert(item)
	utils.MustNotErrorT(t, err)
	v, err := tbl.Get(item.ID)
	utils.MustNotErrorT(t, err)
	utils.MustEqualT(t, item, v)

	item = newStringItem()
	t.Log(item.PrimaryKey())
	err = tbl.Insert(item)
	utils.MustNotErrorT(t, err)
	items = append(items, item)

	l, err := tbl.ListAll()
	t.Log(len(l), err)
	utils.MustNotErrorT(t, err, "ListAll")
	utils.MustNotEmptyT(t, len(l), "ListAll")
	utils.MustEqualT(t, items, l, "ListAll")

	l, err = tbl.ListGreaterThan("\x01", 10)
	utils.MustNotErrorT(t, err, "ListGreaterThan")
	utils.MustEqualT(t, len(l), 2, "ListGreaterThan")

	l, err = tbl.ListLessThan("\xFF", 10)
	utils.MustNotErrorT(t, err)
	utils.MustEqualT(t, 2, len(l), "ListLessThan")
	//t.Log(l[0].ID, l[1].ID)
	utils.MustTrueT(t, l[0].ID < l[1].ID, "ListLessThan")

	err = tbl.Delete(item.ID)
	utils.MustNotErrorT(t, err)

	v, err = tbl.Get(item.ID)
	utils.MustErrorT(t, err)
	utils.MustEqualT(t, true, errors.Is(err, sql.ErrNoRows))
}
