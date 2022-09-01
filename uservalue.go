package thirdlib

import (
	"fmt"
	"reflect"

	"go.starlark.net/starlark"
)

var l = map[reflect.Type]bool{}

type UserValue struct {
	value  interface{} // could be Array/Chan/Map/Slice/Struct/Ptr/Func
	rvalue reflect.Value
	rtype  reflect.Type
	thread *starlark.Thread
}

func NewUserValue(value interface{}, thread *starlark.Thread) *UserValue {
	return &UserValue{value: value,
		rvalue: reflect.ValueOf(value),
		rtype:  reflect.TypeOf(value),
		thread: thread,
	}
}

func (u *UserValue) Interface() interface{} {
	return u.value
}

func (u *UserValue) String() string {
	return fmt.Sprintf("%v", u.value)
}

func (u *UserValue) Type() string {
	return reflect.TypeOf(u.value).String()
}

func (u *UserValue) Freeze() {
}

func (u *UserValue) Truth() starlark.Bool {
	return starlark.Bool(reflect.ValueOf(u.value).IsNil())
}

func (u *UserValue) Hash() (uint32, error) {
	// todo return object hash
	return 0, fmt.Errorf("unhashable")
}

func (u *UserValue) Name() string {
	switch u.rtype.Kind() {
	case reflect.Func:
		return u.rtype.Name()
	default:
		panic(unsupportedError{Type: u.rtype, Method: "Name"})
	}
}

func (u *UserValue) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(kwargs) > 0 {
		return starlark.None, unsupportedError{Type: u.rtype, Method: "Call with kwargs"}
	}
	if u.rvalue.Kind() == reflect.Func {
		var argValues []reflect.Value
		for i := 0; i < u.rtype.NumIn(); i++ {
			inType := u.rtype.In(i)
			if i == u.rtype.NumIn()-1 && u.rtype.IsVariadic() {
				variadicArgs := args[i:]
				for _, variadicArg := range variadicArgs {
					v, err := sValueToReflect(thread, variadicArg, inType.Elem())
					if err != nil {
						return starlark.None, err
					}
					argValues = append(argValues, v)
				}

			} else {
				v, err := sValueToReflect(thread, args[i], inType)
				if err != nil {
					return starlark.None, err
				}
				argValues = append(argValues, v)
			}
		}
		retValues := u.rvalue.Call(argValues)
		var ret []starlark.Value
		for index := range retValues {
			ret = append(ret, ToValue(retValues[index].Interface()))
		}
		if len(ret) == 0 {
			return starlark.None, nil
		} else if len(ret) == 1 {
			return ret[0], nil
		}
		return starlark.Tuple(ret), nil
	}
	return starlark.None, unsupportedError{Type: u.rtype, Method: "Call"}
}

func (u *UserValue) Iterate() starlark.Iterator {
	if u.rvalue.Kind() == reflect.Map {
		return &userValueIterator{m: u.rvalue.MapRange()}
	}
	return &userValueIterator{l: u}
}

type userValueIterator struct {
	l *UserValue
	m *reflect.MapIter
	i int
}

func (it *userValueIterator) Next(p *starlark.Value) bool {
	if it.m != nil {
		if it.m.Next() {
			*p = ToValue(it.m.Key().Interface())
			return true
		}
		return false
	}

	if it.i < it.l.Len() {
		*p = it.l.Index(it.i)
		it.i++
		return true
	}
	return false
}

func (it *userValueIterator) Done() {
}

func (u *UserValue) Slice(start, end, step int) starlark.Value {
	if u.rtype.Kind() != reflect.Slice && u.rtype.Kind() != reflect.Array {
		panic(unsupportedError{Type: u.rtype, Method: "Slice"})
	}
	v := u.rvalue.Slice3(start, end, step)
	var values []starlark.Value
	for i := 0; i < v.Len(); i++ {
		values = append(values, ToValue(v.Index(i)))
	}
	return starlark.NewList(values)
}

func (u *UserValue) SetIndex(index int, v starlark.Value) error {
	if u.rtype.Kind() != reflect.Slice && u.rtype.Kind() != reflect.Array && u.rtype.Kind() != reflect.String {
		return unsupportedError{Type: u.rtype, Method: "SetIndex"}
	}
	typeHint := u.rvalue.Elem().Type()
	if v, err := sValueToReflect(u.thread, v, typeHint); err != nil {
		return err
	} else {
		u.rvalue.Index(index).Set(v)
	}
	return nil
}

func (u *UserValue) Get(k starlark.Value) (v starlark.Value, found bool, err error) {
	if u.rvalue.Kind() == reflect.Map {
		keyType := u.rtype.Key()
		key, err := sValueToReflect(u.thread, k, keyType)
		if err != nil {
			return starlark.None, false, err
		}
		value := u.rvalue.MapIndex(key)
		if !value.IsValid() {
			return starlark.None, false, nil
		}
		return ToValue(value.Interface()), true, nil
	}
	return starlark.None, false, unsupportedError{Type: u.rtype, Method: "Get"}
}

func (u *UserValue) Items() (ret []starlark.Tuple) {
	if u.rvalue.Kind() == reflect.Map {
		iter := u.rvalue.MapRange()
		for iter.Next() {
			ret = append(ret, starlark.Tuple{
				ToValue(iter.Key()), ToValue(iter.Value()),
			})
		}
		return
	}
	panic(unsupportedError{Type: u.rtype, Method: "Items"})
}

func (u *UserValue) SetKey(k, v starlark.Value) error {
	if u.rvalue.Kind() == reflect.Map {
		keyType := u.rtype.Key()
		elemType := u.rtype.Elem()
		lKey, err := sValueToReflect(u.thread, k, keyType)
		if err != nil {
			return err
		}
		lValue, err := sValueToReflect(u.thread, v, elemType)
		if err != nil {
			return err
		}
		u.rvalue.SetMapIndex(lKey, lValue)
		return nil
	}
	panic(unsupportedError{Type: u.rtype, Method: "SetKey"})
}

func (u *UserValue) Attr(name string) (starlark.Value, error) {
	m := u.rvalue.MethodByName(name)
	if m.IsValid() {
		return ToValue(m.Interface()), nil
	}

	rvalue := u.rvalue
	if rvalue.Kind() == reflect.Ptr {
		rvalue = rvalue.Elem()
	}
	if rvalue.Kind() == reflect.Struct {
		field := rvalue.FieldByName(name)
		if !field.IsValid() || !field.CanInterface() {
			return starlark.None, unsupportedError{Type: u.rtype, Method: "Attr: " + name}
		}
		if (field.Kind() == reflect.Struct || field.Kind() == reflect.Array) && field.CanAddr() {
			field = field.Addr()
		}
		return ToValue(field.Interface()), nil
	}
	return starlark.None, unsupportedError{Type: u.rtype, Method: "Attr"}
}

func (u *UserValue) AttrNames() (ret []string) {
	// todo attrNames should return methodNames?
	rvalue := u.rvalue
	if rvalue.Kind() == reflect.Ptr {
		rvalue = rvalue.Elem()
	}
	if rvalue.Kind() == reflect.Struct {
		for i := 0; i < rvalue.NumField(); i++ {
			ret = append(ret, u.rtype.Field(i).Name)
		}
	}
	return ret
}

func (u *UserValue) SetField(name string, val starlark.Value) error {
	// todo SetField could set method?
	rvalue := u.rvalue
	if rvalue.Kind() == reflect.Ptr {
		rvalue = rvalue.Elem()
	}
	if rvalue.Kind() == reflect.Struct {
		field := rvalue.FieldByName(name)
		value, err := sValueToReflect(u.thread, val, field.Type())
		if err != nil {
			return err
		}
		field.Set(value)
		return nil
	}
	return unsupportedError{Type: u.rtype, Method: "SetField"}
}

func (u *UserValue) Index(i int) starlark.Value {
	switch u.rtype.Kind() {
	case reflect.Array, reflect.Slice:
		return ToValue(u.rvalue.Index(i).Interface())
	case reflect.Ptr:
		switch u.rtype.Elem().Kind() {
		case reflect.Array:
			return ToValue(u.rvalue.Elem().Index(i).Interface())
		}
	}
	panic(unsupportedError{Type: u.rtype, Method: "Index"})
}

func (u *UserValue) Len() int {
	switch u.rtype.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.Chan:
		return u.rvalue.Len()
	case reflect.Ptr:
		switch u.rtype.Elem().Kind() {
		case reflect.Array, reflect.Map, reflect.Slice, reflect.Chan:
			return u.rvalue.Elem().Len()
		}
	}
	panic(unsupportedError{Type: u.rtype, Method: "Len"})
}
