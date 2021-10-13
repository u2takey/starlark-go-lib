package thirdlib

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/go-resty/resty/v2"
)

type Greet struct {
	Name string
}

func (t *Greet) Hello() string {
	return "hello: <" + t.Name + ">"
}

func (t *Greet) HelloTo(a string) string {
	return "hello: <" + a + ">"
}

func (t *Greet) CopyFrom(b *Greet) *Greet {
	t.Name = b.Name
	return t
}

func (t *Greet) RenameWithFunc(f func(a string) string) *Greet {
	t.Name = f(t.Name)
	return t
}

func NewGreet() *Greet {
	return &Greet{}
}

func NewGreetWith(name string) *Greet {
	return &Greet{Name: name}
}

func InstallAllExampleModule(d starlark.StringDict) {
	for _, v := range exampleModules {
		d[v.Name] = v
	}
}

var GreetModule = &starlarkstruct.Module{
	Name: "greet",
	Members: starlark.StringDict{
		"new":         ToValue(NewGreet),
		"default":     ToValue(&Greet{}),
		"newWithName": ToValue(NewGreetWith),
	},
}

type M = map[string]interface{}
type E = []M

var exampleModules = []*starlarkstruct.Module{
	GreetModule,
	{
		Name: "modules",
		Members: starlark.StringDict{
			"all": ToValue(func() (ret []string) {
				for _, v := range starlark.Universe {
					if m, ok := v.(*starlarkstruct.Module); ok {
						ret = append(ret, m.Name)
					}
				}
				return
			}),
			"inspect": ToValue(func(a string) (ret []string) {
				if v, ok := starlark.Universe[a]; ok {
					if m, ok := v.(*starlarkstruct.Module); ok {
						for x, y := range m.Members {
							ret = append(ret, fmt.Sprintf("%s: [%s, %s]", x, y.Type(), y.String()))
						}
					}
				}
				return
			}),
		},
	},
	{
		Name: "go",
		Members: starlark.StringDict{
			"new_e":        ToValue(func() E { return E{} }),
			"new_m":        ToValue(func() M { return M{} }),
			"new_e_ptr":    ToValue(func() *E { return &E{} }),
			"new_m_ptr":    ToValue(func() *M { return &M{} }),
			"to_star_type": ToValue(func(a interface{}) starlark.Value { return DecodeValue(a) }),
		},
	},
	{
		Name: "http",
		Members: starlark.StringDict{
			"get":           ToValue(http.Get),
			"pos":           ToValue(http.Post),
			"defaultClient": ToValue(http.DefaultClient),
		},
	},
	{
		Name: "ioutil",
		Members: starlark.StringDict{
			"read_all":   ToValue(ioutil.ReadAll),
			"read_file":  ToValue(os.ReadFile),
			"write_file": ToValue(os.WriteFile),
			"read_dir":   ToValue(os.ReadDir),
		},
	},
	{
		Name: "strings",
		Members: starlark.StringDict{
			"contains": ToValue(strings.Contains),
			"split":    ToValue(strings.Split),
		},
	},
	{
		Name: "context",
		Members: starlark.StringDict{
			"background": ToValue(context.Background),
		}}, {
		Name: "regexp",
		Members: starlark.StringDict{
			"compile":      ToValue(regexp.Compile),
			"match":        ToValue(regexp.Match),
			"match_string": ToValue(regexp.MatchString),
		},
	},
	{
		Name: "url",
		Members: starlark.StringDict{
			"parse": ToValue(url.Parse),
		},
	},
	{
		Name: "exec",
		Members: starlark.StringDict{
			"cmd": ToValue(exec.Command),
			"run": ToValue(func(a ...string) ([]byte, []byte, error) {
				out, err := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
				cmd := exec.Command(a[0], a[1:]...)
				cmd.Stdout, cmd.Stderr = out, err
				err1 := cmd.Run()
				return out.Bytes(), err.Bytes(), err1
			}),
		},
	},
	{
		Name: "resty",
		Members: starlark.StringDict{
			"new": ToValue(resty.New),
		},
	},
}
