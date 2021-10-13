# starlark-go-lib

starlark-go-lib not only provide some libs for starlark-go, but also provide a simple method to create modules from go-lib to starlark-go lib.


## examples to create modules

```go
package httplib

import (
	"net/http"

	thirdlib "github.com/u2takey/starlark-go-lib"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

var httpModules = &starlarkstruct.Module{
	Name: "http",
	Members: starlark.StringDict{
		"get":           thirdlib.ToValue(http.Get),
		"pos":           thirdlib.ToValue(http.Post),
		"defaultClient": thirdlib.ToValue(http.DefaultClient),
	},
}
```

## examples to use created modules

```bash
➜ ./starlark -recursion -set -globalreassign -lambda
Welcome to Starlark (go.starlark.net)
>>> resp, err = http.get("http://baidu.com")
>>> resp.StatusCode
200
```