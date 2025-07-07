arbiter
-------

[![CI](https://github.com/enova/arbiter/actions/workflows/ci.yml/badge.svg)](https://github.com/enova/arbiter/actions/workflows/ci.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/enova/arbiter.svg)](https://pkg.go.dev/github.com/enova/arbiter)

Arbiter is a tool to browse terraform outputs. It provides a web UI for browsing
through terraform state in directory format. It's designed to be able to work
with a selection of different named storage "backends". The main interface is a
configurable `http.Handler` so you can embed or run it as you deem necessary.

## Example Usage

Here's an example of using arbiter with a local filesystem.

```go
package main

import (
        "log"
        "net/http"
        "os"

        "github.com/enova/arbiter"
)

func main() {
	state := os.DirFS(os.Getenv("TF_STATE_DIR"))

	backends := arbiter.NewBackendList()
	backends.AddState("local", state)

	h, err := arbiter.NewHandler(backends, log.Default())
	if err != nil {
		panic(err)
	}

	log.Fatal(http.ListenAndServe(os.Getenv("ADDR"), h))
}
```

If you would rather serve arbiter from a nested path, make sure to use
`http.StripPrefix` like so

```go
	h, err := arbiter.NewHandler(backends, log.Default())
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/arbiter/", http.StripPrefix("/arbiter", h))

	log.Fatal(http.ListenAndServe(os.Getenv("ADDR"), mux))
```

### Backend JSON Format

The `BackendListFromJSON` function can be used to initialize a list of backends
based on a JSON configuration. Currently s3 is the only supported backend.
Implementation uses [s3fs](https://github.com/packrat386/s3fs) and expects
credentials to be available in either env vars or configuration files in
accordance with the aws go SDK v2.

Assuming you had a local `config.json` with the following contents:

```json
[
    {
        "name": "production",
        "type": "s3",
        "connection_info": {
            "bucket_name": "my-production-bucket"
        }
    },
    {
        "name": "staging",
        "type": "s3",
        "connection_info": {
            "bucket_name": "my-staging-bucket",
            "role_arn": "role-for-use-by-sts"
        }
    }
]
```

You could read it like so

```go
package main

import (
	"fmt"
	"os"

	"github.com/enova/arbiter"
)

func main() {
	f, err := os.Open("./config.json")
	if err != nil {
		panic(err)
	}

	backends, err := arbiter.BackendListFromJSON(f)
	if err != nil {
		panic(err)
	}

	for _, name := range backends.Names() {
		fmt.Println("registered: ", name)
	}
}
```

## Screenshots

For a simple terraform state with two sub-directories, prod and staging

Top level search page:

![Top Level](screenshots/arbiter_dir.png?raw=true "Top Level")

Outputs in a sub directory:

![Outputs](screenshots/arbiter_outputs.png?raw=true "Outputs")


## Render Format

The default mode for using arbiter is in the browser, where it renders
HTML. It can also render its search results as JSON for easier machine
consumption. For example:

```
  $ curl -s 'http://localhost:6060/search?backend=be_1&spath=foo&format=json' | jq
{
  "backendNames": [
    "be_1",
    "be_2"
  ],
  "result": {
    "outputs": {
      "name": "foo"
    },
    "terraformVersion": "1.9.1",
    "subdirs": {}
  },
  "spath": "foo",
  "selectedBackend": "be_1"
}
```

If the queried search path has available subdirectories, they will be
listed in the `result` key. For example:

```
  $ curl -s 'http://localhost:6060/search?backend=be_1&spath=.&format=json' | jq
{
  "backendNames": [
    "be_1",
    "be_2"
  ],
  "result": {
    "outputs": null,
    "terraformVersion": "",
    "subdirs": {
      "bar": "/search?backend=be_1&spath=bar",
      "foo": "/search?backend=be_1&spath=foo"
    }
  },
  "spath": ".",
  "selectedBackend": "be_1"
}
```

In the event of an error the `"error"` key will be set at the top
level. For example:

```
  $ curl -s 'http://localhost:6060/search?backend=be_1&spath=baz&format=json' | jq
{
  "backendNames": [
    "be_1",
    "be_2"
  ],
  "error": "could not read path contents: open baz: no such file or directory",
  "result": {
    "outputs": null,
    "terraformVersion": "",
    "subdirs": null
  },
  "spath": "baz",
  "selectedBackend": "be_1"
}
```

The arguments `backend` and `spath` are the backend and search path to
query (default is first available backend and a spath of `.`), and the
`format` can be either `html` (which is the default if none is
specified) or `json`, which will output the structure from the
example.

Support for JSON rendering is experimental, and the format is subject
to change. Specifying any `format` other than `html` or `json` will
result in an error message.

## Assumptions

The main assumption arbiter makes is that all of your outputs, including those
marked sensitive, are safe to read. If you need access control you can implement
it as a middleware, but arbiter doesn't natively have any.
