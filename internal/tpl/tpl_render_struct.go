package tpl

import (
	"reflect"
	"strings"
	"sync"
)

const (
	templateStructSuffix = "Template"
	templateNameSuffix   = "_view"
)

type structTypeMeta struct {
	byName map[string][]int
}

var structMetaCache sync.Map // map[reflect.Type]*structTypeMeta

func chunkTemplateName(v any) string {
	if v == nil {
		return ""
	}
	rt := reflect.TypeOf(v)
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	name := strings.TrimSuffix(rt.Name(), templateStructSuffix)
	if name == "" {
		return ""
	}
	return pascalToSnake(name) + templateNameSuffix
}

func pascalToSnake(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				prev := s[i-1]
				prevLowerOrDigit := (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9')
				nextLower := i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z'
				if prevLowerOrDigit || ((prev >= 'A' && prev <= 'Z') && nextLower) {
					b.WriteByte('_')
				}
			}
			b.WriteByte(c + ('a' - 'A'))
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func registerStructField(sf reflect.StructField, idx []int, byName map[string][]int) {
	if sf.PkgPath != "" {
		return
	}
	if _, exists := byName[sf.Name]; !exists {
		byName[sf.Name] = idx
	}
}

type structWalkItem struct {
	typ    reflect.Type
	prefix []int
}

func collectStructFields(rootType reflect.Type, byName map[string][]int) {
	queue := []structWalkItem{{typ: rootType, prefix: nil}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		n := cur.typ.NumField()
		for i := 0; i < n; i++ {
			sf := cur.typ.Field(i)
			if !sf.Anonymous {
				idx := append(append([]int(nil), cur.prefix...), i)
				registerStructField(sf, idx, byName)
			}
		}
		for i := 0; i < n; i++ {
			sf := cur.typ.Field(i)
			if sf.Anonymous {
				idx := append(append([]int(nil), cur.prefix...), i)
				subType := sf.Type
				if subType.Kind() == reflect.Pointer {
					subType = subType.Elem()
				}
				if subType.Kind() == reflect.Struct && len(idx) <= 4 {
					queue = append(queue, structWalkItem{typ: subType, prefix: idx})
				}
				registerStructField(sf, idx, byName)
			}
		}
	}
}

func getStructTypeMeta(t reflect.Type) *structTypeMeta {
	if cached, ok := structMetaCache.Load(t); ok {
		return cached.(*structTypeMeta)
	}
	byName := make(map[string][]int, t.NumField())
	collectStructFields(t, byName)
	meta := &structTypeMeta{
		byName: byName,
	}
	structMetaCache.Store(t, meta)
	return meta
}

func resolveStructFieldIndex(rv reflect.Value, idx []int) (any, bool) {
	if len(idx) == 1 {
		return rv.Field(idx[0]).Interface(), true
	}
	v := rv
	for _, step := range idx {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return "", true
			}
			v = v.Elem()
		}
		v = v.Field(step)
	}
	return v.Interface(), true
}

func lookupStructField(curr any, part string) (any, bool) {
	if curr == nil {
		return nil, false
	}
	rv := reflect.ValueOf(curr)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", true
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, false
	}
	meta := getStructTypeMeta(rv.Type())
	if idx, ok := meta.byName[part]; ok {
		return resolveStructFieldIndex(rv, idx)
	}
	return nil, false
}

func isStructOrSliceTruthy(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return rv.Len() > 0
	case reflect.Pointer, reflect.Interface:
		return !rv.IsNil()
	}
	return true
}

func reflectSliceLen(v any) int {
	if v == nil {
		return 0
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		return rv.Len()
	}
	return 0
}

func reflectSliceItemAt(v any, i int) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		return rv.Index(i).Interface()
	}
	return nil
}
