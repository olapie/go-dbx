package rt

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestIndirectKind(t *testing.T) {
	t.Run("Nil", func(t *testing.T) {
		k := IndirectKind(nil)
		if diff := cmp.Diff(reflect.Invalid, k); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Struct", func(t *testing.T) {
		var p time.Time
		k := IndirectKind(p)
		if diff := cmp.Diff(reflect.Struct, k); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("PointerToStruct", func(t *testing.T) {
		var p *time.Time
		k := IndirectKind(p)
		if diff := cmp.Diff(reflect.Struct, k); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("PointerToPointerToStruct", func(t *testing.T) {
		var p **time.Time
		k := IndirectKind(p)
		if diff := cmp.Diff(reflect.Struct, k); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Map", func(t *testing.T) {
		var p map[string]any
		k := IndirectKind(p)
		if diff := cmp.Diff(reflect.Map, k); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("PointerToMap", func(t *testing.T) {
		var p map[string]any
		k := IndirectKind(p)
		if diff := cmp.Diff(reflect.Map, k); diff != "" {
			t.Fatal(diff)
		}
	})
}
