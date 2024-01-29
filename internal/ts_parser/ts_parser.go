package ts_parser

// the typescript parser will be a singleton initialized at startup
// so we can import it from anywhere without having to pass it around

type InterestingKinds string

const (
	ExportNamedDeclaration   InterestingKinds = "ExportNamedDeclaration"
	ExportDefaultDeclaration InterestingKinds = "ExportDefaultDeclaration"
	ExportAllDeclaration     InterestingKinds = "ExportAllDeclaration"
	ImportDeclaration        InterestingKinds = "ImportDeclaration"
)

type BodyItem struct {
	Type  InterestingKinds `json:"type"`
	Start uint32           `json:"start"`
	End   uint32           `json:"end"`
}

type ParserReturnBody []BodyItem
type TypescriptParser func(string) ParserReturnBody

type tsParserSingleton struct {
	Parse TypescriptParser
}

var instantiated *tsParserSingleton = nil

func Get() *tsParserSingleton {
	if instantiated == nil {
		instantiated = new(tsParserSingleton)
	}

	return instantiated
}

func (t *tsParserSingleton) SetParser(parser TypescriptParser) {
	if t.Parse != nil {
		t.Parse = parser
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
