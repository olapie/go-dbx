package sqlutil

import (
	"reflect"
	"regexp"
	"strings"
	"sync"
	"unsafe"

	"go.olapie.com/conv"
)

var _regexpVariable = regexp.MustCompile("^[_a-zA-Z]\\w*$")
var _bytesType = reflect.TypeOf([]byte(nil))
var _int64Type = reflect.TypeOf(int64(0))
var _typeToColumnInfo = &sync.Map{} //type:*columnInfo
var _sqlKeywords = map[string]struct{}{
	"primary":        {},
	"key":            {},
	"auto_increment": {},
	"insert":         {},
	"create":         {},
	"table":          {},
	"database":       {},
	"select":         {},
	"update":         {},
	"unique":         {},
	"int":            {},
	"bigint":         {},
	"bool":           {},
	"tinyint":        {},
	"double":         {},
	"date":           {},
	"json":           {},
	"nullable":       {},
}

type fieldIndex []int

func (f fieldIndex) DeepEqual(v fieldIndex) bool {
	return reflect.DeepEqual(f, v)
}

func (f fieldIndex) Equal(v fieldIndex) bool {
	s1 := (*reflect.SliceHeader)(unsafe.Pointer(&f))
	s2 := (*reflect.SliceHeader)(unsafe.Pointer(&v))
	return s1.Data == s2.Data
}

type columnInfo struct {
	//indexes of fields without tag db:"-"
	indexes []fieldIndex

	//column names
	names []string

	nameToIndex map[string]fieldIndex

	//primary key column names
	pkNames []string

	//auto increment column name
	aiName string

	jsonNames []string

	nullableNames []string

	//for speed
	notPKNames []string
	notAINames []string
}

func getColumnInfo(typ reflect.Type) *columnInfo {
	if i, ok := _typeToColumnInfo.Load(typ); ok {
		return i.(*columnInfo)
	}

	if typ.Kind() != reflect.Struct {
		panic("not struct")
	}

	info := parseColumnInfo(typ)
	_typeToColumnInfo.Store(typ, info)
	return info
}

func parseColumnInfo(typ reflect.Type) *columnInfo {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return nil
	}

	info := &columnInfo{}
	info.nameToIndex = make(map[string]fieldIndex, typ.NumField())

	fields := getAllFields(typ)

	for _, f := range fields {
		tag := strings.TrimSpace(strings.ToLower(f.Tag.Get("sql")))
		if tag == "-" {
			continue
		}

		if f.Name[0] < 'A' || f.Name[0] > 'Z' {
			if len(tag) > 0 {
				panic("sql column must be exported field: " + f.Name)
			}
			continue
		}

		isJSON := strings.Contains(tag, "json")
		nullable := strings.Contains(tag, "nullable")

		if !isJSON && !isSupportType(f.Type) {
			if len(tag) > 0 {
				panic("invalid type: db column " + typ.Name() + ":" + f.Type.String())
			}
			continue
		}

		var name string
		if len(tag) > 0 {
			strs := strings.Split(tag, ",")
			if len(strs) > 0 {
				if _, ok := _sqlKeywords[strs[0]]; !ok && _regexpVariable.MatchString(strs[0]) {
					name = strs[0]
				}
			}
		}

		if len(name) == 0 {
			name = conv.ToSnake(f.Name)
		}

		if idx, found := info.nameToIndex[name]; found {
			if len(idx) < len(f.Index) {
				continue
			}

			if len(idx) == len(f.Index) {
				panic("duplicate column name:" + name)
			}
		}

		if strings.Contains(tag, "primary key") {
			if isJSON {
				panic("json column can't be primary key")
			}
			info.pkNames = append(info.pkNames, name)
		}

		if strings.Contains(tag, "auto_increment") {
			if len(info.aiName) > 0 {
				panic("duplicate auto_increment")
			}

			if !f.Type.ConvertibleTo(_int64Type) {
				panic("not integer: " + f.Type.String())
			}
			info.aiName = name
		}

		info.indexes = append(info.indexes, f.Index)
		info.names = append(info.names, name)
		info.nameToIndex[name] = f.Index
		if isJSON {
			info.jsonNames = append(info.jsonNames, name)
		}

		if nullable {
			info.nullableNames = append(info.nullableNames, name)
		}
	}

	if len(info.pkNames) == 0 {
		for _, name := range info.names {
			if name == "id" {
				info.pkNames = []string{name}
				break
			}
		}
	}

	for _, name := range info.names {
		if IndexOfString(info.pkNames, name) < 0 {
			info.notPKNames = append(info.notPKNames, name)
		}

		if name != info.aiName {
			info.notAINames = append(info.notAINames, name)
		}
	}

	if len(info.aiName) > 0 && (IndexOfString(info.pkNames, info.aiName) != 0 || len(info.pkNames) != 1) {
		panic("auto_increment must be used with primary key")
	}

	return info
}

func isSupportType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}

	switch typ.Kind() {
	case reflect.Bool, reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.String:
		return true
	default:
		if typ.ConvertibleTo(_bytesType) {
			return true
		}
	}

	return false
}

func getAllFields(typ reflect.Type) []reflect.StructField {
	fields := make([]reflect.StructField, 0)
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Anonymous {
			t := f.Type
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			subFields := getAllFields(t)
			for i := range subFields {
				subFields[i].Index = append([]int{i}, subFields[i].Index...)
			}
			fields = append(fields, subFields...)
		} else {
			fields = append(fields, f)
		}
	}

	return fields
}
