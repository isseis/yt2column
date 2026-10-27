//go:build test

package articlecache

import (
	"fmt"
	"reflect"
	"strings"
)

// DumpRecordsForTest renders every record the cache holds, every field
// included and every pointer followed, so a test can check that no record
// holds a given string. It walks the record type by reflection, so a field
// added to record later is rendered too.
func (c *Cache) DumpRecordsForTest() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var b strings.Builder
	for key, rec := range c.records {
		fmt.Fprintf(&b, "%s: ", key)
		dumpValue(&b, reflect.ValueOf(rec).Elem())
		b.WriteByte('\n')
	}
	return b.String()
}

// dumpValue writes v, reading unexported fields through reflect's accessors,
// which allow reads of them.
func dumpValue(b *strings.Builder, v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
		dumpValue(b, v.Elem())
	case reflect.Struct:
		b.WriteByte('{')
		for i := range v.NumField() {
			fmt.Fprintf(b, "%s:", v.Type().Field(i).Name)
			dumpValue(b, v.Field(i))
			b.WriteByte(' ')
		}
		b.WriteByte('}')
	case reflect.Slice, reflect.Array:
		b.WriteByte('[')
		for i := range v.Len() {
			dumpValue(b, v.Index(i))
			b.WriteByte(' ')
		}
		b.WriteByte(']')
	case reflect.Map:
		b.WriteByte('{')
		iter := v.MapRange()
		for iter.Next() {
			dumpValue(b, iter.Key())
			b.WriteByte(':')
			dumpValue(b, iter.Value())
			b.WriteByte(' ')
		}
		b.WriteByte('}')
	case reflect.String:
		b.WriteString(v.String())
	case reflect.Bool:
		fmt.Fprint(b, v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprint(b, v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		fmt.Fprint(b, v.Uint())
	case reflect.Float32, reflect.Float64:
		fmt.Fprint(b, v.Float())
	default:
		// Functions, channels, and unsafe pointers hold no text.
		b.WriteString(v.Kind().String())
	}
}
