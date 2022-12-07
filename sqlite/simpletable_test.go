package sqlite_test

import (
	"database/sql"
	"testing"

	"errors"

	"code.olapie.com/sqlx/sqlite"
	"code.olapie.com/sugar/types"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func createTable[K sqlite.IntOrString, M sqlite.PrimaryKey[K]](t *testing.T, name string, newModel func() M) *sqlite.SimpleTable[K, M] {
	db, err := sql.Open("sqlite3", "file::memory:")
	if err != nil {
		t.Error(err)
	}

	tbl, err := sqlite.NewSimpleTable[K](db, name, newModel)
	require.NoError(t, err)
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
		ID:    types.RandomID().Int(),
		Name:  types.RandomID().Pretty(),
		Score: float64(types.RandomID()) / float64(3),
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
		ID:    types.RandomID().Pretty(),
		Name:  types.RandomID().Pretty(),
		Score: float64(types.RandomID()) / float64(3),
	}
}

func TestIntTable(t *testing.T) {
	tbl := createTable[int64](t, "tbl"+types.RandomID().Pretty(), func() *IntItem { return new(IntItem) })
	var items []*IntItem
	item := newIntItem()
	items = append(items, item)
	err := tbl.Insert(item)
	require.NoError(t, err)
	v, err := tbl.Get(item.ID)
	require.NoError(t, err)
	require.Equal(t, item, v)

	item = newIntItem()
	item.ID = items[0].ID + 1
	err = tbl.Insert(item)
	require.NoError(t, err)
	items = append(items, item)

	l, err := tbl.ListAll()
	require.NoError(t, err)
	require.NotEmpty(t, l)
	require.Equal(t, items, l)

	l, err = tbl.ListGreaterThan(item.ID, 10)
	require.NoError(t, err)
	require.Empty(t, l)

	l, err = tbl.ListLessThan(item.ID+1, 10)
	require.NoError(t, err)
	require.Equal(t, 2, len(l))
	//t.Log(l[0].ID, l[1].ID)
	require.True(t, l[0].ID < l[1].ID)

	err = tbl.Delete(item.ID)
	require.NoError(t, err)

	v, err = tbl.Get(item.ID)
	require.NotEmpty(t, err)
	require.Equal(t, true, errors.Is(err, sql.ErrNoRows))
}

func TestStringTable(t *testing.T) {
	tbl := createTable[string](t, "tbl"+types.RandomID().Pretty(), func() *StringItem { return new(StringItem) })
	var items []*StringItem
	item := newStringItem()
	items = append(items, item)
	err := tbl.Insert(item)
	require.NoError(t, err)
	v, err := tbl.Get(item.ID)
	require.NoError(t, err)
	require.Equal(t, item, v)

	item = newStringItem()
	err = tbl.Insert(item)
	require.NoError(t, err)
	items = append(items, item)

	l, err := tbl.ListAll()
	require.NoError(t, err)
	require.NotEmpty(t, l)
	require.Equal(t, items, l)

	l, err = tbl.ListGreaterThan("a", 10)
	require.NoError(t, err)
	require.Empty(t, l)

	l, err = tbl.ListLessThan("Z", 10)
	require.NoError(t, err)
	require.Equal(t, 2, len(l))
	//t.Log(l[0].ID, l[1].ID)
	require.True(t, l[0].ID < l[1].ID)

	err = tbl.Delete(item.ID)
	require.NoError(t, err)

	v, err = tbl.Get(item.ID)
	require.NotEmpty(t, err)
	require.Equal(t, true, errors.Is(err, sql.ErrNoRows))
}
