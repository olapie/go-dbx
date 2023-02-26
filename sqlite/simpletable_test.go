package sqlite

import (
	"database/sql"
	"errors"
	"math/rand"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"go.olapie.com/dbx/internal/testutil"
)

func createTable[K SimpleKey, R SimpleTableRecord[K]](t *testing.T, name string) *SimpleTable[K, R] {
	t.Log("createTable")

	db, err := sql.Open("sqlite3", "file::memory:")
	if err != nil {
		t.Error(err)
	}

	tbl, err := NewSimpleTable[K, R](db, name)
	testutil.NoError(t, err)
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
	tbl := createTable[int64, *IntItem](t, "tbl"+uuid.NewString())
	var items []*IntItem
	item := newIntItem()
	items = append(items, item)
	err := tbl.Insert(item)
	testutil.NoError(t, err)
	v, err := tbl.Get(item.ID)
	testutil.NoError(t, err)
	testutil.Equal(t, item, v)

	item = newIntItem()
	item.ID = items[0].ID + 1
	err = tbl.Insert(item)
	testutil.NoError(t, err)
	items = append(items, item)

	l, err := tbl.ListAll()
	testutil.NoError(t, err)
	testutil.True(t, len(l) != 0)
	testutil.Equal(t, items, l)

	l, err = tbl.ListGreaterThan(item.ID, 10)
	testutil.NoError(t, err)
	testutil.True(t, len(l) == 0)

	l, err = tbl.ListLessThan(item.ID+1, 10)
	testutil.NoError(t, err)
	testutil.Equal(t, 2, len(l))
	//t.Log(l[0].ID, l[1].ID)
	testutil.True(t, l[0].ID < l[1].ID)

	err = tbl.Delete(item.ID)
	testutil.NoError(t, err)

	v, err = tbl.Get(item.ID)
	testutil.Error(t, err)
	testutil.Equal(t, true, errors.Is(err, sql.ErrNoRows))
}

func TestStringTable(t *testing.T) {
	t.Log("TestStringTable")

	tbl := createTable[string, *StringItem](t, "tbl"+uuid.NewString())
	var items []*StringItem
	item := newStringItem()
	items = append(items, item)
	err := tbl.Insert(item)
	testutil.NoError(t, err)
	v, err := tbl.Get(item.ID)
	testutil.NoError(t, err)
	testutil.Equal(t, item, v)

	item = newStringItem()
	err = tbl.Insert(item)
	testutil.NoError(t, err)
	items = append(items, item)

	l, err := tbl.ListAll()
	testutil.NoError(t, err)
	testutil.True(t, len(l) != 0)
	testutil.Equal(t, items, l)

	l, err = tbl.ListGreaterThan("a", 10)
	testutil.NoError(t, err)
	testutil.True(t, len(l) == 0)

	l, err = tbl.ListLessThan("Z", 10)
	testutil.NoError(t, err)
	testutil.Equal(t, 2, len(l))
	//t.Log(l[0].ID, l[1].ID)
	testutil.True(t, l[0].ID < l[1].ID)

	err = tbl.Delete(item.ID)
	testutil.NoError(t, err)

	v, err = tbl.Get(item.ID)
	testutil.Error(t, err)
	testutil.Equal(t, true, errors.Is(err, sql.ErrNoRows))
}
