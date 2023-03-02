package sqlutil

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"

	"go.olapie.com/utils"
)

type tableNamer interface {
	TableName() string
}

func getTableName(record any) string {
	if n, ok := record.(tableNamer); ok {
		return n.TableName()
	}

	return getTableNameByType(reflect.TypeOf(record))
}

func getTableNameBySlice(records any) string {
	typ := reflect.TypeOf(records)
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Slice {
		panic("must be a pointer to slice")
	}

	return getTableNameByType(typ.Elem())
}

func getTableNameByType(typ reflect.Type) string {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		panic("not struct: " + typ.String())
	}

	if typ.Implements(_tableNamerType) {
		return reflect.Zero(typ).Interface().(tableNamer).TableName()
	}

	if reflect.PtrTo(typ).Implements(_tableNamerType) {
		// Pointer receiver may be dereferenced during TableName method call
		// New its elem value in order to make pointer non-nil
		return reflect.New(typ).Interface().(tableNamer).TableName()
		//return reflect.Zero(reflect.PtrTo(typ)).Interface().(tableNamer).TableName()
	}

	return utils.Plural(utils.ToSnake(typ.Name()))
}

type Table struct {
	exe        Executor
	driverName string
	name       string
}

func (t *Table) Insert(record any) error {
	query, values, err := t.prepareInsertQuery(record)
	if err != nil {
		log.Println(err)
		return err
	}

	if Debug {
		log.Println(query, toReadableArgs(values))
	}

	result, err := t.exe.Exec(query, values...)
	if err != nil {
		log.Println(err)
		return err
	}
	v := getStructValue(record)
	info := getColumnInfo(v.Type())
	if len(info.aiName) > 0 && v.FieldByIndex(info.nameToIndex[info.aiName]).Int() == 0 {
		id, err := result.LastInsertId()
		if err != nil {
			log.Println(err)
			return err
		}
		v.FieldByIndex(info.nameToIndex[info.aiName]).SetInt(id)
	}

	return nil
}

func (t *Table) prepareInsertQuery(record any) (string, []any, error) {
	v := getStructValue(record)
	info := getColumnInfo(v.Type())

	var columns []string
	values := make([]any, 0, len(info.indexes))
	if len(info.aiName) > 0 && v.FieldByIndex(info.nameToIndex[info.aiName]).Int() == 0 {
		columns = info.notAINames
	} else {
		columns = info.names
	}

	for _, name := range columns {
		fv, err := t.getFieldValueByName(v, info, name)
		if err != nil {
			return "", nil, err
		}
		values = append(values, fv)
	}

	var buf bytes.Buffer
	buf.WriteString("INSERT INTO ")
	buf.WriteString(t.name)
	buf.WriteString("(")
	buf.WriteString(strings.Join(columns, ", "))
	buf.WriteString(") VALUES (")
	buf.WriteString(strings.Repeat("?, ", len(columns)))
	buf.Truncate(buf.Len() - 2)
	buf.WriteString(")")
	return buf.String(), values, nil
}

func (t *Table) Update(record any) error {
	v := getStructValue(record)
	info := getColumnInfo(v.Type())
	if len(info.pkNames) == 0 {
		panic("no primary key. please use Insert operation")
	}

	var buf bytes.Buffer
	buf.WriteString("UPDATE ")
	buf.WriteString(t.name)
	buf.WriteString(" SET ")
	for i, c := range info.notPKNames {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(c)
		buf.WriteString(" = ?")
	}

	buf.WriteString(" WHERE ")
	for i, c := range info.pkNames {
		if i > 0 {
			buf.WriteString(" and ")
		}
		buf.WriteString(c)
		buf.WriteString(" = ?")
	}

	args := make([]any, 0, len(info.indexes))
	for _, name := range info.notPKNames {
		fv, err := t.getFieldValueByName(v, info, name)
		if err != nil {
			return err
		}
		args = append(args, fv)
	}

	for _, name := range info.pkNames {
		args = append(args, v.FieldByIndex(info.nameToIndex[name]).Interface())
	}

	query := buf.String()
	if Debug {
		log.Println(query, toReadableArgs(args))
	}
	_, err := t.exe.Exec(query, args...)
	return err
}

func (t *Table) Save(record any) error {
	switch t.driverName {
	case "mysql":
		return t.mysqlSave(record)
	case "sqlite3":
		return t.sqliteSave(record)
	default:
		panic("Save operation is not supported for driver: " + t.driverName)
	}
}

func (t *Table) mysqlSave(record any) error {
	query, values, err := t.prepareInsertQuery(record)
	if err != nil {
		log.Println(err)
		return err
	}

	v := getStructValue(record)
	info := getColumnInfo(v.Type())

	var buf bytes.Buffer
	buf.WriteString(query)
	buf.WriteString(" ON DUPLICATE KEY UPDATE ")
	for i, name := range info.notPKNames {
		if name == "created_at" {
			continue
		}

		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(name)
		buf.WriteString(" = ?")
		fv, err := t.getFieldValueByName(v, info, name)
		if err != nil {
			return err
		}
		values = append(values, fv)
	}

	query = buf.String()

	if Debug {
		log.Println(query, toReadableArgs(values))
	}

	result, err := t.exe.Exec(query, values...)
	if len(info.aiName) > 0 && v.FieldByIndex(info.nameToIndex[info.aiName]).Int() == 0 {
		id, err := result.LastInsertId()
		if err != nil {
			log.Println(err)
			return err
		}
		v.FieldByIndex(info.nameToIndex[info.aiName]).SetInt(id)
	}
	return err
}

func (t *Table) sqliteSave(record any) error {
	query, values, err := t.prepareInsertQuery(record)
	if err != nil {
		log.Println(err)
		return err
	}

	query = strings.Replace(query, "INSERT INTO", "INSERT OR REPLACE INTO", 1)
	v := getStructValue(record)
	info := getColumnInfo(v.Type())

	if Debug {
		log.Println(query, toReadableArgs(values))
	}

	result, err := t.exe.Exec(query, values...)
	if len(info.aiName) > 0 && v.FieldByIndex(info.nameToIndex[info.aiName]).Int() == 0 {
		id, err := result.LastInsertId()
		if err != nil {
			log.Println(err)
			return err
		}
		v.FieldByIndex(info.nameToIndex[info.aiName]).SetInt(id)
	}
	return err
}

func (t *Table) Select(records any, where string, args ...any) error {
	v := reflect.ValueOf(records)
	if v.Kind() != reflect.Ptr {
		panic("must be a pointer to slice")
	}

	if v.IsNil() && !v.CanSet() {
		panic("cannot be set value")
	}

	sliceType := v.Type().Elem()
	if sliceType.Kind() != reflect.Slice {
		panic("must be a pointer to slice")
	}

	isPtr := false
	elemType := sliceType.Elem()
	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
		isPtr = true
	}

	if elemType.Kind() != reflect.Struct {
		panic("slice element must be a struct or pointer to struct")
	}

	fi := getColumnInfo(elemType)

	var buf bytes.Buffer
	buf.WriteString("SELECT ")
	buf.WriteString(strings.Join(fi.names, ", "))
	buf.WriteString(" FROM ")
	buf.WriteString(t.name)
	if len(where) > 0 {
		buf.WriteString(" WHERE ")
		buf.WriteString(where)
	}
	query := buf.String()

	if Debug {
		log.Println(query, toReadableArgs(args))
	}

	rows, err := t.exe.Query(query, args...)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	if v.IsNil() {
		v.Set(reflect.New(sliceType))
	}
	sliceValue := v.Elem()
	fields := make([]any, len(fi.indexes))
	for rows.Next() {
		ptrToElem := utils.DeepNew(elemType)
		elem := ptrToElem.Elem()
		for i, idx := range fi.indexes {
			if IndexOfString(fi.jsonNames, fi.names[i]) >= 0 {
				var data []byte
				fields[i] = &data
			} else if IndexOfString(fi.nullableNames, fi.names[i]) >= 0 {
				switch elem.FieldByIndex(idx).Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
					reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					var v sql.NullInt64
					fields[i] = &v
				case reflect.Bool:
					var b sql.NullBool
					fields[i] = &b
				case reflect.Float32, reflect.Float64:
					var v sql.NullFloat64
					fields[i] = &v
				case reflect.String:
					var v sql.NullString
					fields[i] = &v
				default:
					panic("invalid nullable type" + fmt.Sprint(elem.FieldByIndex(idx).Type()))
				}
			} else {
				fields[i] = elem.FieldByIndex(idx).Addr().Interface()
			}
		}

		err = rows.Scan(fields...)
		if err != nil {
			log.Println(err)
			return err
		}

		for _, name := range fi.jsonNames {
			idx := fi.nameToIndex[name]
			i := IndexOfString(fi.names, name)
			addr := fields[i]
			data := reflect.ValueOf(addr).Elem().Interface()
			err = json.Unmarshal(data.([]byte), elem.FieldByIndex(idx).Addr().Interface())
			if err != nil {
				log.Println(err)
				return err
			}
		}

		for _, name := range fi.nullableNames {
			idx := fi.nameToIndex[name]
			i := IndexOfString(fi.names, name)
			addr := fields[i]
			switch v := reflect.ValueOf(addr).Elem().Interface().(type) {
			case sql.NullString:
				if v.Valid {
					elem.FieldByIndex(idx).SetString(v.String)
				}
			case sql.NullFloat64:
				if v.Valid {
					elem.FieldByIndex(idx).SetFloat(v.Float64)
				}
			case sql.NullBool:
				if v.Valid {
					elem.FieldByIndex(idx).SetBool(v.Bool)
				}
			case sql.NullInt64:
				if v.Valid {
					elem.FieldByIndex(idx).SetInt(v.Int64)
				}
			default:
				panic("invalid type:" + fmt.Sprint(v))
			}
		}

		if isPtr {
			sliceValue = reflect.Append(sliceValue, ptrToElem)
		} else {
			sliceValue = reflect.Append(sliceValue, elem)
		}
	}
	v.Elem().Set(sliceValue)
	return nil
}

func (t *Table) SelectOne(record any, where string, args ...any) error {
	rv := reflect.ValueOf(record)
	if rv.Kind() != reflect.Ptr {
		panic("not pointer to a struct")
	}

	//Store result in ev. If failed, don't change record's value
	ev := utils.DeepNew(rv.Elem().Type()).Elem()
	elem := ev
	if elem.Kind() == reflect.Ptr {
		elem = elem.Elem()
	}

	if elem.Kind() != reflect.Struct {
		panic("not pointer to a struct")
	}

	info := getColumnInfo(elem.Type())

	var buf bytes.Buffer
	buf.WriteString("SELECT ")
	buf.WriteString(strings.Join(info.names, ", "))
	buf.WriteString(" FROM ")
	buf.WriteString(t.name)
	if len(where) > 0 {
		buf.WriteString(" WHERE ")
		buf.WriteString(where)
	}
	query := buf.String()

	if Debug {
		log.Println(query, toReadableArgs(args))
	}

	fieldAddrs := make([]any, len(info.indexes))
	for i, idx := range info.indexes {
		if IndexOfString(info.jsonNames, info.names[i]) >= 0 {
			var data []byte
			fieldAddrs[i] = &data
		} else if IndexOfString(info.nullableNames, info.names[i]) >= 0 {
			field := elem.FieldByIndex(idx)
			switch {
			case utils.IsInt(field), utils.IsUint(field):
				var v sql.NullInt64
				fieldAddrs[i] = &v
			case field.Kind() == reflect.Bool:
				var b sql.NullBool
				fieldAddrs[i] = &b
			case utils.IsFloat(field):
				var v sql.NullFloat64
				fieldAddrs[i] = &v
			case field.Kind() == reflect.String:
				var v sql.NullString
				fieldAddrs[i] = &v
			default:
				panic("invalid nullable type" + fmt.Sprint(elem.FieldByIndex(idx).Type()))
			}
		} else {
			fieldAddrs[i] = elem.FieldByIndex(idx).Addr().Interface()
		}
	}
	err := t.exe.QueryRow(query, args...).Scan(fieldAddrs...)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println(err)
		}
		return err
	}

	for _, name := range info.jsonNames {
		idx := info.nameToIndex[name]
		i := IndexOfString(info.names, name)
		addr := fieldAddrs[i]
		data := reflect.ValueOf(addr).Elem().Interface()
		err = json.Unmarshal(data.([]byte), elem.FieldByIndex(idx).Addr().Interface())
		if err != nil {
			log.Println(err)
			return err
		}
	}

	for _, name := range info.nullableNames {
		idx := info.nameToIndex[name]
		i := IndexOfString(info.names, name)
		addr := fieldAddrs[i]
		switch v := reflect.ValueOf(addr).Elem().Interface().(type) {
		case sql.NullString:
			if v.Valid {
				elem.FieldByIndex(idx).SetString(v.String)
			}
		case sql.NullFloat64:
			if v.Valid {
				elem.FieldByIndex(idx).SetFloat(v.Float64)
			}
		case sql.NullBool:
			if v.Valid {
				elem.FieldByIndex(idx).SetBool(v.Bool)
			}
		case sql.NullInt64:
			if v.Valid {
				elem.FieldByIndex(idx).SetInt(v.Int64)
			}
		default:
			panic("invalid type:" + fmt.Sprint(v))
		}
	}

	if err == nil {
		rv.Elem().Set(ev)
	}
	return err
}

/*
func (t *Table) QueryRow(query string, args ...any) *Row {
	row := t.exe.QueryRow(query, args...)
	return (*Row)(row)
}

func (t *Table) Query(query string, args ...any) (*Rows, error) {
	rows, err := t.exe.Query(query, args...)
	return (*Rows)(rows), err
}*/

func (t *Table) Delete(where string, args ...any) error {
	if len(where) == 0 {
		panic("where is empty")
	}
	var buf bytes.Buffer
	buf.WriteString("DELETE FROM ")
	buf.WriteString(t.name)
	buf.WriteString(" WHERE ")
	buf.WriteString(where)

	query := buf.String()

	if Debug {
		log.Println(query, toReadableArgs(args))
	}

	_, err := t.exe.Exec(query, args...)
	if err != nil {
		log.Println(err)
	}
	return err
}

func (t *Table) Count(where string, args ...any) (int, error) {
	var buf bytes.Buffer
	buf.WriteString("SELECT COUNT(*) FROM ")
	buf.WriteString(t.name)
	if len(where) > 0 {
		buf.WriteString(" WHERE ")
		buf.WriteString(where)
	}
	query := buf.String()

	if Debug {
		log.Println(query, toReadableArgs(args))
	}

	var count int
	err := t.exe.QueryRow(query, args...).Scan(&count)
	if err != nil {
		log.Println(err)
		return 0, err
	}

	return count, nil
}

func (t *Table) getFieldValueByName(item reflect.Value, info *columnInfo, name string) (any, error) {
	k := item.FieldByIndex(info.nameToIndex[name]).Interface()
	if IndexOfString(info.jsonNames, name) >= 0 {
		data, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}

		if IndexOfString(info.nullableNames, name) >= 0 && IsNil(string(data)) {
			return nil, nil
		} else {
			return data, nil
		}
	} else {
		if IndexOfString(info.nullableNames, name) >= 0 && k == reflect.Zero(reflect.TypeOf(k)).Interface() {
			return nil, nil
		} else {
			return k, nil
		}
	}
}

func toReadableArgs(args []any) []any {
	if Debug {
		readableArgs := make([]any, len(args))
		for i, a := range args {
			if b, ok := a.([]byte); ok {
				readableArgs[i] = string(b)
			} else {
				readableArgs[i] = a
			}
		}
		return readableArgs
	}
	return args
}

func getStructValue(i any) reflect.Value {
	v := reflect.ValueOf(i)
	if !v.IsValid() {
		panic("invalid")
	}

	for v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		panic("not struct: " + v.Kind().String())
	}

	return v
}

func IndexOfString(a []string, s string) int {
	for i, str := range a {
		if str == s {
			return i
		}
	}

	return -1
}
