# Intársia

An open web application, installed on your own infrastructure, that unifies
databases, spreadsheets and APIs and gives the result back as an API, a
spreadsheet or a database. Light by design: one machine, one binary.

> Early development. Today the binary only serves a placeholder page.

## Build from source

Requirements: Go (version in `go.mod`), Node.js 22 or newer and GNU Make.

```sh
make build
./bin/intarsia version
./bin/intarsia serve        # http://127.0.0.1:8080, or --port to change it
```

## Development

| Command | What it does |
| --- | --- |
| `make build` | Builds the web interface and the binary |
| `make test` | Runs the Go and frontend tests |
| `make lint` | Lints Go and the frontend, including the architecture boundaries |
| `make dev` | Starts the server and the interface in development mode |

Run a `make` target before using `go` directly: the Go code embeds the compiled
interface, which `make` builds first.

## License

[Apache 2.0](LICENSE)
