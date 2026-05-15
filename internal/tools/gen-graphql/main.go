// Command gen-graphql wraps genqlient's code generator so it picks up
// the project's pinned dependency versions (notably the modern
// golang.org/x/tools required by Go 1.26+) instead of whatever transient
// versions `go run github.com/Khan/genqlient` would resolve to.
//
// Invoke via `make gen` or `go run ./internal/tools/gen-graphql`.
package main

import (
	"github.com/Khan/genqlient/generate"
)

func main() {
	generate.Main()
}
