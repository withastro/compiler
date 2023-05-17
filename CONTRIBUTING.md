# Contributing

Contributions are welcome to the Go compiler!

## Setup

### Go

[Go][go] `1.19.x` is needed to work with this repo. On Macs, installing via [Homebrew][homebrew] is recommended: `brew install go`. For Windows & Linux, you can [follow Go’s installation guide][go] if you don’t have your own preferred method of package installation.

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

### Running

- Run all tests: `go test -v ./internal/...`
- Run a specific folder of tests: `go test -v ./internal/printer`

### Adding new tests

Adding tests for the tokenizer, scanner, and printer can be found in `internal/token_test.go`, `internal/js_scanner_test.go`, and `internal/printer/printer_test.go`, respectively.

[homebrew]: https://brew.sh/
[go]: https://golang.org/
[go-vscode]: https://marketplace.visualstudio.com/items?itemName=golang.go
[node]: https://nodejs.org/
