package thirdlib

import (
	"fmt"
	"reflect"

	"go.starlark.net/starlark"
)

var gThread *starlark.Thread

// only for decode, userValue not included in return types
func DecodeValue(value interface{}) starlark.Value {
	switch converted := value.(type) {
	case bool:
		return starlark.Bool(converted)
	case float64:
		return starlark.Float(converted)
	case string:
		return starlark.String(converted)
	case int64:
		return starlark.MakeInt(int(converted))
	case int:
		return starlark.MakeInt(converted)
	case int8:
		return starlark.MakeInt(int(converted))
	case int32:
		return starlark.MakeInt(int(converted))
	case int16:
		return starlark.MakeInt(int(converted))
	case uint64:
		return starlark.MakeUint(uint(converted))
	case uint:
		return starlark.MakeUint(converted)
	case uint8:
		return starlark.MakeUint(uint(converted))
	case uint32:
		return starlark.MakeUint(uint(converted))
	case uint16:
		return starlark.MakeUint(uint(converted))
	case []interface{}:
		var values []starlark.Value
		for _, item := range converted {
			values = append(values, DecodeValue(item))
		}
		return starlark.NewList(values)
	case *[]interface{}:
		if converted == nil {
			return starlark.None
		}
		var values []starlark.Value
		for _, item := range *converted {
			values = append(values, DecodeValue(item))
		}
		return starlark.NewList(values)
	case map[string]interface{}:
		d := starlark.NewDict(len(converted))
		for key, item := range converted {
			_ = d.SetKey(DecodeValue(key), DecodeValue(item))
		}
		return d
	case *map[string]interface{}:
		if converted == nil {
			return starlark.None
		}
		d := starlark.NewDict(len(*converted))
		for key, item := range *converted {
			_ = d.SetKey(DecodeValue(key), DecodeValue(item))
		}
		return d
	case nil:
		return starlark.None
	}
	return starlark.None
}

func ToValue(value interface{}) starlark.Value {
	if value == nil {
		return starlark.None
	}
	if val, ok := value.(starlark.Value); ok {
		return val
	}
	switch val := reflect.ValueOf(value); val.Kind() {
	case reflect.Bool:
		return starlark.Bool(val.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return starlark.MakeInt(int(val.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return starlark.MakeUint(uint(val.Uint()))
	case reflect.Float32, reflect.Float64:
		return starlark.Float(val.Float())
	case reflect.String:
		return starlark.String(val.String())
	case reflect.Slice:
		if val.Type().Elem().Kind() == reflect.Uint8 {
			return starlark.Bytes(val.Bytes())
		}
		fallthrough
	case reflect.Chan, reflect.Map, reflect.Ptr, reflect.Func, reflect.Interface:
		if val.IsNil() {
			return starlark.None
		}
		return NewUserValue(val.Interface(), gThread)
	case reflect.Struct:
		return NewUserValue(val.Interface(), gThread)
	default:
		return NewUserValue(val.Interface(), gThread)
	}
}

var (
	refTypeSValue = reflect.TypeOf((*starlark.Value)(nil)).Elem()
)

type unsupportedError struct {
	Type   reflect.Type
	Method string
}

func (s unsupportedError) Error() string {
	return `type ` + s.Type.String() + ` does not support ` + s.Method
}

type conversionError struct {
	Value starlark.Value
	Hint  reflect.Type
}

func (c conversionError) Error() string {
	if c.Value == starlark.None {
		return fmt.Sprintf("cannot use nil as type %s", c.Hint)
	}
	var val interface{}
	if userData, ok := c.Value.(*UserValue); ok {
		val = userData.value
	} else {
		val = c.Value
	}
	return fmt.Sprintf("cannot use %v (type %T) as type %s", val, val, c.Hint)
}

type structFieldError struct {
	Field string
	Type  reflect.Type
}

func (s structFieldError) Error() string {
	return `type ` + s.Type.String() + ` has no field ` + s.Field
}

func sValueToReflect(thread *starlark.Thread, value starlark.Value, typeHint reflect.Type) (reflect.Value, error) {
	visited := make(map[interface{}]reflect.Value)
	return sValueToReflectInner(thread, value, typeHint, visited)
}

func sValueToReflectInner(thread *starlark.Thread, v starlark.Value, hint reflect.Type, visited map[interface{}]reflect.Value) (reflect.Value, error) {
	if hint.Implements(refTypeSValue) {
		return reflect.ValueOf(v), nil
	}

	isPtr := false

	switch converted := v.(type) {
	case *UserValue:
		val := converted.rvalue
		if val.Kind() != reflect.Ptr && hint.Kind() == reflect.Ptr && val.Type() == hint.Elem() {
			newVal := reflect.New(hint.Elem())
			newVal.Elem().Set(val)
			val = newVal
		} else {
			if !val.Type().ConvertibleTo(hint) {
				return reflect.Value{}, conversionError{Value: converted, Hint: hint}
			}
			val = val.Convert(hint)
		}
		return val, nil
	case starlark.NoneType:
		switch hint.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer, reflect.Uintptr:
			return reflect.Zero(hint), nil
		}
		return reflect.Value{}, conversionError{Value: v, Hint: hint}
	case starlark.Bool:
		val := reflect.ValueOf(bool(converted))
		if !val.Type().ConvertibleTo(hint) {
			return reflect.Value{}, conversionError{Value: v, Hint: hint}
		}
		return val.Convert(hint), nil
	case starlark.Bytes:
		val := reflect.ValueOf([]byte(converted))
		if !val.Type().ConvertibleTo(hint) {
			return reflect.Value{}, conversionError{Value: v, Hint: hint}
		}
		return val.Convert(hint), nil
	case starlark.Int:
		if i, ok := converted.Int64(); !ok {
			return reflect.Value{}, conversionError{Value: v, Hint: hint}
		} else {
			val := reflect.ValueOf(int(i))
			if !val.Type().ConvertibleTo(hint) {
				return reflect.Value{}, conversionError{Value: v, Hint: hint}
			}
			return val.Convert(hint), nil
		}
	case starlark.Float:
		val := reflect.ValueOf(float64(converted))
		if !val.Type().ConvertibleTo(hint) {
			return reflect.Value{}, conversionError{Value: v, Hint: hint}
		}
		return val.Convert(hint), nil

	case starlark.String:
		val := reflect.ValueOf(string(converted))
		if !val.Type().ConvertibleTo(hint) {
			return reflect.Value{}, conversionError{Value: v, Hint: hint}
		}
		return val.Convert(hint), nil
	case *starlark.List:
		if hint.Kind() != reflect.Slice {
			return reflect.Value{}, conversionError{Value: v, Hint: hint}
		}
		elmType := hint.Elem()
		val := reflect.MakeSlice(hint, converted.Len(), converted.Len())
		for i := 0; i < converted.Len(); i++ {
			vi, err := sValueToReflect(thread, converted.Index(i), elmType)
			if err != nil {
				panic(err)
			}
			val.Index(i).Set(vi)
		}
		return val, nil
	case starlark.Tuple:
		if hint.Kind() != reflect.Slice {
			return reflect.Value{}, conversionError{
				Value: v,
				Hint:  hint,
			}
		}
		elmType := hint.Elem()
		val := reflect.MakeSlice(hint, converted.Len(), converted.Len())
		for i := 0; i < converted.Len(); i++ {
			vi, err := sValueToReflect(thread, converted.Index(i), elmType)
			if err != nil {
				panic(err)
			}
			val.Index(i).Set(vi)
		}
		return val, nil
	case *starlark.Set:
		if hint.Kind() != reflect.Slice {
			return reflect.Value{}, conversionError{Value: v, Hint: hint}
		}
		elmType := hint.Elem()
		val := reflect.MakeSlice(hint, converted.Len(), converted.Len())
		iter := converted.Iterate()
		var elem starlark.Value
		for i := 0; iter.Next(&elem); i++ {
			vi, err := sValueToReflect(thread, elem, elmType)
			if err != nil {
				panic(err)
			}
			val.Index(i).Set(vi)
		}
		return val, nil
	case *starlark.Dict:
		if existing := visited[converted]; existing.IsValid() {
			return existing, nil
		}

		switch {

		case hint.Kind() == reflect.Map:
			keyType := hint.Key()
			elemType := hint.Elem()
			s := reflect.MakeMap(hint)
			visited[converted] = s
			items := converted.Items()
			for _, elem := range items {
				key, value := elem[0], elem[1]
				lKey, err := sValueToReflectInner(thread, key, keyType, visited)
				if err != nil {
					return reflect.Value{}, err
				}
				lValue, err := sValueToReflectInner(thread, value, elemType, visited)
				if err != nil {
					return reflect.Value{}, err
				}
				s.SetMapIndex(lKey, lValue)
			}
			return s, nil
		case hint.Kind() == reflect.Ptr && hint.Elem().Kind() == reflect.Struct:
			hint = hint.Elem()
			isPtr = true
			fallthrough
		case hint.Kind() == reflect.Struct:
			s := reflect.New(hint)
			visited[converted] = s
			t := s.Elem()
			items := converted.Items()
			for _, elem := range items {
				key, value := elem[0], elem[1]
				fieldName := ""
				if val, ok := key.(starlark.String); !ok {
					continue
				} else {
					fieldName = val.GoString()
				}
				fieldVal := t.FieldByName(fieldName)
				if !fieldVal.IsValid() {
					return reflect.Value{}, structFieldError{Field: fieldName, Type: hint}
				}
				lValue, err := sValueToReflectInner(thread, value, fieldVal.Type(), visited)
				if err != nil {
					return reflect.Value{}, err
				}
				fieldVal.Set(lValue)
			}
			if isPtr {
				return s, nil
			}
			return t, nil
		}

		return reflect.Value{}, conversionError{Value: v, Hint: hint}
	case *starlark.Function:
		if hint.Kind() != reflect.Func {
			return reflect.Value{}, conversionError{Value: v, Hint: hint}
		}
		if converted == nil {
			return reflect.Zero(hint), nil
		}
		fn := func(args []reflect.Value) (ret []reflect.Value) {
			if converted == nil {

			}
			var tArgs []starlark.Value
			for index := range args {
				tArgs = append(tArgs, ToValue(args[index].Interface()))
			}
			value, err := converted.CallInternal(thread, tArgs, nil)
			if err != nil {
				panic(err)
			}
			if value == starlark.None && hint.NumOut() != 0 {
				panic(conversionError{Value: converted, Hint: hint})
			}
			if tuple, ok := value.(starlark.Tuple); ok {
				for index := range tuple {
					ret0, err := sValueToReflect(thread, tuple[index], hint.Out(index))
					if err != nil {
						panic(err)
					}
					ret = append(ret, ret0)
				}
			} else {
				ret0, err := sValueToReflect(thread, value, hint.Out(0))
				if err != nil {
					panic(err)
				}
				ret = append(ret, ret0)
			}
			return
		}
		return reflect.MakeFunc(hint, fn), nil
	case *starlark.Builtin:
		if hint.Kind() != reflect.Func {
			return reflect.Value{}, conversionError{Value: v, Hint: hint}
		}
		if converted == nil {
			return reflect.Zero(hint), nil
		}
		fn := func(args []reflect.Value) (ret []reflect.Value) {
			if converted == nil {

			}
			var tArgs []starlark.Value
			for index := range args {
				tArgs = append(tArgs, ToValue(args[index].Interface()))
			}
			value, err := converted.CallInternal(thread, tArgs, nil)
			if err != nil {
				panic(err)
			}
			if value == starlark.None && hint.NumOut() != 0 {
				panic(conversionError{Value: converted, Hint: hint})
			}
			if tuple, ok := value.(starlark.Tuple); ok {
				for index := range tuple {
					ret0, err := sValueToReflect(thread, tuple[index], hint.Out(index))
					if err != nil {
						panic(err)
					}
					ret = append(ret, ret0)
				}
			} else {
				ret0, err := sValueToReflect(thread, value, hint.Out(0))
				if err != nil {
					panic(err)
				}
				ret = append(ret, ret0)
			}
			return
		}
		return reflect.MakeFunc(hint, fn), nil
	}

	panic("never reaches")
}
