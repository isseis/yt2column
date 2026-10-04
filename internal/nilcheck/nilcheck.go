// Package nilcheck reports whether a value is nil, including a typed nil held
// in an interface, so that constructors can reject a missing dependency no
// matter how the caller spelled it.
package nilcheck

import "reflect"

// IsNil reports whether v is a nil interface value or holds a nil pointer,
// func, map, slice, or channel.
func IsNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}
