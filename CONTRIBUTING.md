# Contributing

Contributions are welcome to the Go compiler!

## Setup

### Go

[Go][go] `1.20+` is needed to work with this repo. On Macs, installing via [Homebrew][homebrew] is recommended: `brew install go`. For Windows & Linux, you can [follow Go’s installation guide][go] if you don’t have your own preferred method of package installation.

If you use VS Code as your primary editor, installing the [Go extension][go-vscode] is highly recommended.

### Node

You will also need [Node.js][node] installed, as well as PNPM 8.x (`npm i -g pnpm`). More often than not, you won’t need to touch JS in this repo, but in case you do, be sure to run `pnpm install` first.

## Code Structure

A simple explanation of the compiler process is:

1. Tokenizes (`internal/token.go`)
2. Scans (`internal/js_scanner.go`)
3. Prints (`internal/printer/print-to-js.go`)

**Tokenizing** takes the raw `.astro` text and turns it into simple tokens such as `FrontmatterStart`, `FrontmatterEnd`, `TagStart`, `TagEnd`, etc.

**Scanning** does a basic scanning of the JS to pull out imports after the tokenizer has made it clear where JS begins and ends.

**Printing** takes all the output up till now and generates (prints) valid TypeScript that can be executed within Node.

When adding a new feature or debugging an issue, start at the tokenizer, then move onto the scanner, and finally end at the printer. By starting at the lowest level of complexity (tokenizer), it will be easier to reason about.

## Tests

It's important to **run the test from the root of the project**. Doing so, `go` will load all the necessary global information needed to run the tests.

### Run all tests

```shell
go test -v ./internal/...
```
### Run a specific test suite 

```shell
go test -v ./internal/printer
```
### Run a specific test case

Many of your test cases are designed like this:

```go
func TestPrintToJSON(t *testing.T) {
  tests := []jsonTestcase{
  	{
  	  name:   "basic",
  	  source: `<h1>Hello world!</h1>`,
  	  want:   []ASTNode{{Type: "element", Name: "h1", Children: []ASTNode{{Type: "text", Value: "Hello world!"}}}},
  	},
    {
  	  name:   "Comment preserves whitespace",
  	  source: `<!-- hello -->`,
  	  want:   []ASTNode{{Type: "comment", Value: " hello "}},
  	}
  }
}
```

In this particular instance, the test case is name of the function, a slash `/`, followed by the `name` field. If the test case has spaces, you can use them.

```shell
go test -v ./internal/... -run TestPrintToJSON/basic
go test -v ./internal/... -run TestPrintToJSON/Comment preserves whitespace
```

### Adding new tests

Adding tests for the tokenizer, scanner, and printer can be found in `internal/token_test.go`, `internal/js_scanner_test.go`, and `internal/printer/printer_test.go`, respectively.

### Snapshot testing

We use [go-snaps](https://github.com/gkampitakis/go-snaps) for snapshot testing. Visit their repository for more details on how to use it

#### Update snapshots

Some of our tests use snapshot tests. If some of you changes are expected to update some snapshot tests, you can use the environment variable `UPDATE_SNAPS` to do so:

```shell
UPDATE_SNAPS=true go test -v ./internal/...
```

Instead, if there are some **obsolete snapshots**, you can `UPDATE_SNAPS=clean`:

```shell
UPDATE_SNAPS=clean go test -v ./internal/...
```

[homebrew]: https://brew.sh/
[go]: https://golang.org/
[go-vscode]: https://marketplace.visualstudio.com/items?itemName=golang.go
[node]: https://nodejs.org/
