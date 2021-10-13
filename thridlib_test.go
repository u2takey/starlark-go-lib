package thirdlib

import (
	"testing"

	"go.starlark.net/starlark"
)

func TestEvalGreetLib(t *testing.T) {
	thread := new(starlark.Thread)
	InstallAllExampleModule(starlark.Universe)
	for _, test := range []struct{ src, want string }{
		{`greet.new().Hello()`, `"hello: <>"`},
		{`greet.newWithName("tom").Hello()`, `"hello: <tom>"`},
		{`greet.new().RenameWithFunc(lambda a: "tom").Hello()`, `"hello: <tom>"`},
		{`greet.new().RenameWithFunc(lambda a: a+"tom").Hello()`, `"hello: <tom>"`},
		{`resty.new().SetCookie({"Name": "a", "Value": "b"})`, ``},
	} {
		var got string
		if v, err := starlark.Eval(thread, "<expr>", test.src, nil); err != nil {
			got = err.Error()
		} else {
			got = v.String()
		}
		if got != test.want {
			t.Errorf("eval %s = %s, want %s", test.src, got, test.want)
		}
	}
}
