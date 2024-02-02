package ts_parser

import (
	"context"
	"embed"
	"encoding/json"
	"log"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// the typescript parser will be a singleton initialized at startup
// so we can import it from anywhere without having to pass it around

type interestingKinds string

const (
	ExportNamedDeclaration   interestingKinds = "ExportNamedDeclaration"
	ExportDefaultDeclaration interestingKinds = "ExportDefaultDeclaration"
	ExportAllDeclaration     interestingKinds = "ExportAllDeclaration"
	ImportDeclaration        interestingKinds = "ImportDeclaration"
)

type BodyItem struct {
	Type  interestingKinds `json:"type"`
	Start int              `json:"start"`
	End   int              `json:"end"`
}

type ParserReturn struct {
	Body []BodyItem `json:"body"`
}

var parserSingleton typescriptParser
var cleanupSingleton func()

/*
A function that returns a parser function and a cleanup function

The cleanup function is used to free-up memory allocated by the parser.
It should only be called when the parser is no longer needed.
*/
func GetParser() (typescriptParser, func()) {
	if parserSingleton == nil {
		parserSingleton, cleanupSingleton = createTypescriptParser()
	}
	return parserSingleton, cleanupSingleton
}

type typescriptParser func(string) ParserReturn

//go:embed wasm/*.wasm
var wasmFolder embed.FS

func createTypescriptParser() (typescriptParser, func()) {
	ctx := context.Background()
	r := wazero.NewRuntime(ctx)

	wasmBytes, _ := wasmFolder.ReadFile("wasm/ts_parser.wasm")

	mod, err := r.Instantiate(ctx, wasmBytes)
	if err != nil {
		log.Panicf("failed to instantiate module: %v", err)
	}

	printAst := mod.ExportedFunction("print_ast")
	allocate := mod.ExportedFunction("allocate")
	deallocate := mod.ExportedFunction("deallocate")

	parser := createParserFunction(&ctx, &allocate, &deallocate, &printAst, &mod)

	cleanup := func() {
		r.Close(ctx)
		parserSingleton = nil
	}
	return parser, cleanup
}

func createParserFunction(ctx *context.Context, allocate *api.Function, deallocate *api.Function, printAst *api.Function, mod *api.Module) func(string) ParserReturn {
	return func(sourceText string) ParserReturn {
		sourceTextSize := uint64(len(sourceText))

		// Instead of an arbitrary memory offset, use Rust's allocator. Notice
		// there is nothing string-specific in this allocation function. The same
		// function could be used to pass binary serialized data to Wasm.
		results, err := (*allocate).Call(*ctx, sourceTextSize)

		if err != nil {
			log.Panicln(err)
		}

		sourceTextPtr := results[0]
		defer (*deallocate).Call(*ctx, sourceTextPtr, sourceTextSize)

		// This pointer was allocated by Rust, but owned by Go, So, we have to
		// deallocate it when finished
		// defer deallocate.Call(ctx, sourceTextPtr, sourceTextSize)

		if !(*mod).Memory().Write(uint32(sourceTextPtr), []byte(sourceText)) {
			log.Panicf("Memory.Write(%d, %d) out of range of memory size %d",
				sourceTextPtr, sourceTextSize, (*mod).Memory().Size())
		}

		// Now, we can call "print_ast", which reads the string we wrote to memory!
		ptrSize, err := (*printAst).Call(*ctx, sourceTextPtr, sourceTextSize)
		if err != nil {
			log.Panicln(err)
		}

		astPtr := uint32(ptrSize[0] >> 32)
		astSize := uint32(ptrSize[0])
		defer (*deallocate).Call(*ctx, uint64(astPtr), uint64(astSize))

		bytes, ok := (*mod).Memory().Read(astPtr, astSize)
		// The pointer is a linear memory offset, which is where we write the name.
		if !ok {
			log.Panicf("Memory.Read(%d, %d) out of range of memory size %d",
				astPtr, astSize, (*mod).Memory().Size())
		}

		// fmt.Printf("Returned ast: %s\n", string(bytes))

		var ast ParserReturn
		error := json.Unmarshal(bytes, &ast)
		if error != nil {
			log.Panicln(error)
		}

		return ast
	}
}

//////////////////////////////////////////////
// type ModuleKind string

// const (
// 	Script ModuleKind = "script"
// 	Module ModuleKind = "module"
// )

// type Hava string
// type TypeScriptLanguage struct{
// 	isDefinitionFile bool
// }

// const (
// 	JavaScript string = "javaScript"
// 	TypeScript TypeScriptLanguage =
// )

// type Program struct{
// 	Span
// 	sourceType
// }

// type Error struct{}
// type Trivias struct{}

// type SourceType struct {
//     /// JavaScript or TypeScript, default JavaScript
//     language Language

//     /// Script or Module, default Module
//     moduleKind ModuleKind

//     /// Support JSX for JavaScript and TypeScript? default without JSX
//     variant LanguageVariant

//     /// Mark strict mode as always strict
//     /// See <https://github.com/tc39/test262/blob/main/INTERPRETING.md#strict-mode>
//     alwaysStrict bool
// }

// type ParserReturn struct {
// 	program  Program
// 	errors   []Error
// 	trivias  Trivias
// 	panicked bool
// }
