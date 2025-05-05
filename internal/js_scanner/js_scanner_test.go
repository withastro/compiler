package js_scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/withastro/compiler/internal/test_utils"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/ast"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/core"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/parser"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/scanner"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/tspath"
)

type testcase struct {
	name   string
	source string
	want   string
	only   bool
}

func fixturesHoistImport() []testcase {
	return []testcase{
		{
			name:   "basic",
			source: `const value = "test"`,
			want:   ``,
		},
		{
			name: "import",
			source: `import { fn } from "package";
const b = await fetch();`,
			want: `import { fn } from "package";
`,
		},
		{
			name: "dynamic",
			source: `const markdownDocs = await Astro.glob('../markdown/*.md')
const article2 = await import('../markdown/article2.md')
`,
			want: "",
		},
		{
			name: "big import",
			source: `import {
a,
b,
c,
d,
} from "package"

const b = await fetch();`,
			want: `import {
a,
b,
c,
d,
} from "package"
`,
		},
		{
			name: "import with comment",
			source: `// comment
import { fn } from "package";
const b = await fetch();`,
			want: `// comment
import { fn } from "package";`,
		},
		{
			name: "import assertion",
			source: `// comment
import { fn } from "package" assert { it: 'works' };
const b = await fetch();`,
			want: `// comment
import { fn } from "package" assert { it: 'works' };`,
		},
		{
			name: "import assertion 2",
			source: `// comment
import {
fn
} from
"package" assert {
it: 'works'
};
const b = await fetch();`,
			want: `// comment
import {
fn
} from
"package" assert {
it: 'works'
};
`,
		},
		{
			name: "import.meta.env",
			source: `console.log(import.meta.env.FOO);
import Test from "../components/Test.astro";`,
			want: `import Test from "../components/Test.astro";`,
		},
		{
			name: "import.meta.env II",
			source: `console.log(
import
	.meta
	.env
	.FOO
);
import Test from "../components/Test.astro";`,
			want: `import Test from "../components/Test.astro";`,
		},
		{
			name: "import/export",
			source: `import { fn } from "package";
export async fn() {}
const b = await fetch()`,
			want: `import { fn } from "package";`,
		},
		{
			name: "getStaticPaths",
			source: `import { fn } from "package";
export async function getStaticPaths() {
const content = Astro.fetchContent('**/*.md');
}
const b = await fetch()`,
			want: `import { fn } from "package";`,
		},
		{
			name: "getStaticPaths with comments",
			source: `import { fn } from "package";
export async function getStaticPaths() {
const content = Astro.fetchContent('**/*.md');
}
const b = await fetch()`,
			want: `import { fn } from "package";`,
		},
		{
			name: "getStaticPaths with semicolon",
			source: `import { fn } from "package";
export async function getStaticPaths() {
const content = Astro.fetchContent('**/*.md');
}; const b = await fetch()`,
			want: `import { fn } from "package";`,
		},
		{
			name: "getStaticPaths with RegExp escape",
			source: `export async function getStaticPaths() {
const pattern = /\.md$/g.test('value');
}
import a from "a";`,
			want: `import a from "a";`,
		},
		{
			name: "getStaticPaths with divider",
			source: `export async function getStaticPaths() {
const pattern = a / b;
}`,
			want: ``,
		},
		{
			name: "getStaticPaths with divider and following content",
			source: `export async function getStaticPaths() {
const value = 1 / 2;
}
// comment
import { b } from "b";
const { a } = Astro.props;`,
			want: `// comment
import { b } from "b";`,
		},
		{
			name: "getStaticPaths with regex and following content",
			source: `export async function getStaticPaths() {
const value = /2/g;
}
// comment
import { b } from "b";
const { a } = Astro.props;`,
			want: `// comment
import { b } from "b";`,
		},
		{
			name: "multiple imports",
			source: `import { a } from "a";
import { b } from "b";
// comment
import { c } from "c";
const d = await fetch()

// comment
import { d } from "d";`,
			want: `import { a } from "a";
import { b } from "b";
// comment
import { c } from "c";
// comment
import { d } from "d";
`,
		},
		{
			name:   "assignment",
			source: `let show = true;`,
			want:   ``,
		},
		{
			name: "RegExp is not a comment",
			source: `import { a } from "a";
/import \{ b \} from "b";/;
import { c } from "c";`,
			want: `import { a } from "a";
import { c } from "c";
`,
		},
	}
}

func TestHoistImport(t *testing.T) {
	tests := fixturesHoistImport()
	for _, tt := range tests {
		if tt.only {
			tests = make([]testcase, 0)
			tests = append(tests, tt)
			break
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewScanner([]byte(tt.source))
			result := s.Result.ImportsInfo
			got := []byte{}
			for _, imp := range result.Data {
				got = append(got, bytes.TrimSpace(imp)...)
				got = append(got, '\n')
			}
			// compare to expected string, show diff if mismatch
			if diff := test_utils.ANSIDiff(strings.TrimSpace(tt.want), strings.TrimSpace(string(got))); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEmptySuited(t *testing.T) {
	src := `
		// ------------------------------------INCLUDED------------------------------------
    import foo from "./foo";
    import { namedImport1, namedImport2 } from "./named";
    import * as namespace from "./namespace";
    import "./side-effect";
    import type { TypeImport } from "./types";
		import { default as DefaultImport } from "./default";
		import { default as DefaultImport } from "./default" with { assert: { type: "json" } };


    export const y: string = "hello";
		export { namedExport1, namedExport2 } from "./named";
		export * as namespace from "./namespace";
		export type { TypeExport } from "./types";
		export default function() {};
		export { default as DefaultExport } from "./default";

		// import equals declarations with export modifier
		export import type B = require("B");
		export import C = require("B");

		// namespace alias with export modifier
		export import a = foo.a

		// ------------------------------------EXCLUDED------------------------------------
		import("dynamic").then((dynamicImport) => {});
		const dynamicImport = await import("./dynamic");

		// export assignment, d.ts specific I believe
		export = 1;

		// import equals declarations
		import type A = require("A");
		import B = require("A");

		// namespace alias
		import a = foo.a
`
	// use an absolute‐style path for parser
	fileName := "/test.ts"

	start := time.Now()
	path := tspath.Path(fileName)
	// parse with ESNext + full JSDoc mode
	sf := parser.ParseSourceFile(fileName, path, src, core.ScriptTargetESNext, scanner.JSDocParsingModeParseAll)
	node := sf.AsNode()
	imports := make([]*ast.Node, 0)
	exports := make([]*ast.Node, 0)

	var visitor ast.Visitor
	visitor = func(node *ast.Node) bool {
		// if the node is nil, stop traversing
		if node == nil {
			return true
		}

		fmt.Printf("Node type: %s\n", node.Kind)

		// Check for imports
		isImport := ast.IsImportDeclaration(node)

		if isImport {
			imports = append(imports, node)
			// get to the next node
			return false
		}

		// Check for exports (existing logic)
		isExport := ast.IsExportDeclaration(node) || ast.HasSyntacticModifier(node, ast.ModifierFlagsExport)

		if isExport {
			exports = append(exports, node)
			// get to the next node
			return false
		}

		// Continue traversing
		return node.ForEachChild(visitor)
	}

	node.ForEachChild(visitor)
	elapsed := time.Since(start)

	fmt.Println("Imports found:", len(imports))
	for i, imp := range imports {
		text := src[imp.Pos():imp.End()]
		fmt.Printf("Import %d\nText: `%s`\nPos: %d\n\n", i+1, text, imp.Pos())
	}

	fmt.Println("\n------------------")

	fmt.Println("\nExports found:", len(exports))
	for i, exp := range exports {
		text := src[exp.Pos():exp.End()]
		fmt.Printf("Export %d\nText: `%s`\nPos: %d\n\n", i+1, text, exp.Pos())
	}

	fmt.Printf("\nParsing took %s\n", elapsed)
}

func FuzzHoistImport(f *testing.F) {
	tests := fixturesHoistImport()
	for _, tt := range tests {
		f.Add(tt.source) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T, source string) {
		s := NewScanner([]byte(source))
		result := s.Result.ImportsInfo
		got := []byte{}
		for _, imp := range result.Data {
			got = append(got, bytes.TrimSpace(imp)...)
			got = append(got, '\n')
		}
		if utf8.ValidString(source) && !utf8.ValidString(string(got)) {
			t.Errorf("Import hoisting produced an invalid string: %q", got)
		}
	})
}

func TestHoistExport(t *testing.T) {
	tests := []testcase{
		{
			name: "getStaticPaths",
			source: `import { fn } from "package";
export async function getStaticPaths() {
	const content = Astro.fetchContent('**/*.md');
}
const b = await fetch()`,
			want: `export async function getStaticPaths() {
	const content = Astro.fetchContent('**/*.md');
}`,
		},
		{
			name: "getStaticPaths with comments",
			source: `import { fn } from "package";
export async function getStaticPaths() {
  // This works!
  const content = Astro.fetchContent('**/*.md');
}
const b = await fetch()`,
			want: `export async function getStaticPaths() {
  // This works!
  const content = Astro.fetchContent('**/*.md');
}`,
		},
		{
			name: "getStaticPaths with semicolon",
			source: `import { fn } from "package";
export async function getStaticPaths() {
  const content = Astro.fetchContent('**/*.md');
}; const b = await fetch()`,
			want: `export async function getStaticPaths() {
  const content = Astro.fetchContent('**/*.md');
}`,
		},
		{
			name: "getStaticPaths with RegExp escape",
			source: `// cool
export async function getStaticPaths() {
  const pattern = /\.md$/g.test('value');
}
import a from "a";`,
			want: `// cool
export async function getStaticPaths() {
  const pattern = /\.md$/g.test('value');
}`,
		},
		{
			name: "getStaticPaths with divider",
			source: `export async function getStaticPaths() {
  const pattern = a / b;
}`,
			want: `export async function getStaticPaths() {
  const pattern = a / b;
}`,
		},
		{
			name: "getStaticPaths with divider and following content",
			source: `export async function getStaticPaths() {
  const value = 1 / 2;
}
// comment
import { b } from "b";
const { a } = Astro.props;`,
			want: `export async function getStaticPaths() {
  const value = 1 / 2;
}`,
		},
		{
			name: "getStaticPaths with regex and following content",
			source: `// comment
export async function getStaticPaths() {
  const value = /2/g;
}
import { b } from "b";
const { a } = Astro.props;`,
			want: `// comment
export async function getStaticPaths() {
  const value = /2/g;
}`,
		},
		{
			name: "getStaticPaths with TypeScript type",
			source: `import { fn } from "package";

export async function getStaticPaths({
	paginate
}: {
	paginate: any
}) {
	const content = Astro.fetchContent('**/*.md');
}
const b = await fetch()`,
			want: `export async function getStaticPaths({
	paginate
}: {
	paginate: any
}) {
	const content = Astro.fetchContent('**/*.md');
}`,
		},
		{
			name: "export interface",
			source: `import { a } from "a";
export interface Props {
	open?: boolean;
}`,
			want: `export interface Props {
	open?: boolean;
}`,
		},
		{
			name: "export multiple",
			source: `import { a } from "a";
export interface Props {
	open?: boolean;
}
export const foo = "bar"`,
			want: `export interface Props {
	open?: boolean;
}
export const foo = "bar"`,
		},
		{
			name: "export multiple with content after",
			source: `import { a } from "a";
export interface Props {
	open?: boolean;
}
export const baz = "bing"
// beep boop`,
			want: `export interface Props {
	open?: boolean;
}
export const baz = "bing"`,
		},
		{
			name: "export three",
			source: `import { a } from "a";
export interface Props {}
export const a = "b"
export const c = "d"`,
			want: `export interface Props {}
export const a = "b"
export const c = "d"`,
		},
		{
			name: "export with comments",
			source: `import { a } from "a";
// comment
export interface Props {}
export const a = "b"
export const c = "d"`,
			want: `// comment
export interface Props {}
export const a = "b"
export const c = "d"`,
		},
		{
			name: "export local reference (runtime error)",
			source: `import { a } from "a";
export interface Props {}
const value = await fetch("something")
export const data = { value }
`,
			want: `export interface Props {}
export const data = { value }`,
		},
		{
			name: "export passthrough",
			source: `export * from "./local-data.json";
export { default as A } from "./_types"
// Not valid JS
// export B from "./_types"
// export type C from "./_types"`,
			want: `export * from "./local-data.json";
export { default as A } from "./_types"`,
		},
		{
			name: "multi-line export",
			source: `export interface Props
{
	foo: 'bar';
}`,
			want: `export interface Props
{
	foo: 'bar';
}`,
		},
		{
			name: "multi-line type export",
			source: `export type Props =
{
	foo: 'bar';
}`,
			want: `export type Props =
{
	foo: 'bar';
}`,
		},
		{
			name: "multi-line type export with multiple exports",
			source: `export type Theme = 'light' | 'dark';

export type Props =
{
	theme: Theme;
};

export interface Foo {
	bar: string;
}

export type FooAndBar1 = 'Foo' &
'Bar';
export type FooAndBar2 = 'Foo'
& 'Bar';
export type FooOrBar = 'Foo'
| 'Bar';`,
			want: `export type Theme = 'light' | 'dark';
export type Props =
{
	theme: Theme;
};
export interface Foo {
	bar: string;
}
export type FooAndBar1 = 'Foo' &
'Bar';
export type FooAndBar2 = 'Foo'
& 'Bar';
export type FooOrBar = 'Foo'
| 'Bar';`,
		},
		{
			name: "Picture",
			source: `// @ts-ignore
import loader from 'virtual:image-loader';
import { getPicture } from '../src/get-picture.js';
import type { ImageAttributes, ImageMetadata, OutputFormat, PictureAttributes, TransformOptions } from '../src/types.js';
export interface LocalImageProps extends Omit<PictureAttributes, 'src' | 'width' | 'height'>, Omit<TransformOptions, 'src'>, Omit<ImageAttributes, 'src' | 'width' | 'height'> {
	src: ImageMetadata | Promise<{ default: ImageMetadata }>;
	sizes: HTMLImageElement['sizes'];
	widths: number[];
	formats?: OutputFormat[];
}
export interface RemoteImageProps extends Omit<PictureAttributes, 'src' | 'width' | 'height'>, TransformOptions, Omit<ImageAttributes, 'src' | 'width' | 'height'> {
	src: string;
	sizes: HTMLImageElement['sizes'];
	widths: number[];
	aspectRatio: TransformOptions['aspectRatio'];
	formats?: OutputFormat[];
}
export type Props = LocalImageProps | RemoteImageProps;
const { src, sizes, widths, aspectRatio, formats = ['avif', 'webp'], loading = 'lazy', decoding = 'async', ...attrs } = Astro.props as Props;
const { image, sources } = await getPicture({ loader, src, widths, formats, aspectRatio });
`,
			want: `export interface LocalImageProps extends Omit<PictureAttributes, 'src' | 'width' | 'height'>, Omit<TransformOptions, 'src'>, Omit<ImageAttributes, 'src' | 'width' | 'height'> {
	src: ImageMetadata | Promise<{ default: ImageMetadata }>;
	sizes: HTMLImageElement['sizes'];
	widths: number[];
	formats?: OutputFormat[];
}
export interface RemoteImageProps extends Omit<PictureAttributes, 'src' | 'width' | 'height'>, TransformOptions, Omit<ImageAttributes, 'src' | 'width' | 'height'> {
	src: string;
	sizes: HTMLImageElement['sizes'];
	widths: number[];
	aspectRatio: TransformOptions['aspectRatio'];
	formats?: OutputFormat[];
}
export type Props = LocalImageProps | RemoteImageProps;`,
		},
		{
			name: "Image",
			source: `// @ts-ignore
import loader from 'virtual:image-loader';
import { getImage } from '../src/index.js';
import type { ImageAttributes, ImageMetadata, TransformOptions, OutputFormat } from '../src/types.js';
const { loading = "lazy", decoding = "async", ...props } = Astro.props as Props;
const attrs = await getImage(loader, props);

// Moved after Astro.props for test
export interface LocalImageProps extends Omit<TransformOptions, 'src'>, Omit<ImageAttributes, 'src' | 'width' | 'height'> {
	src: ImageMetadata | Promise<{ default: ImageMetadata }>;
}
export interface RemoteImageProps extends TransformOptions, ImageAttributes {
	src: string;
	format: OutputFormat;
	width: number;
	height: number;
}
export type Props = LocalImageProps | RemoteImageProps;
`,
			want: `// Moved after Astro.props for test
export interface LocalImageProps extends Omit<TransformOptions, 'src'>, Omit<ImageAttributes, 'src' | 'width' | 'height'> {
	src: ImageMetadata | Promise<{ default: ImageMetadata }>;
}
export interface RemoteImageProps extends TransformOptions, ImageAttributes {
	src: string;
	format: OutputFormat;
	width: number;
	height: number;
}
export type Props = LocalImageProps | RemoteImageProps;`,
		},
		{
			name: "comments",
			source: `//
export const foo = 0
/*
*/`,
			want: `//
export const foo = 0`,
		},
	}

	for _, tt := range tests {
		if tt.only {
			tests = make([]testcase, 0)
			tests = append(tests, tt)
			break
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewScanner([]byte(tt.source))

			result := s.Result.ExportsInfo
			got := []byte{}
			for _, imp := range result.Data {
				got = append(got, bytes.TrimSpace(imp)...)
				got = append(got, '\n')
			}
			// compare to expected string, show diff if mismatch
			if diff := test_utils.ANSIDiff(strings.TrimSpace(tt.want), strings.TrimSpace(string(got))); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type keytestcase struct {
	name   string
	source string
	want   []string
	only   bool
}

func TestGetObjectKeys(t *testing.T) {
	tests := []keytestcase{
		{
			name:   "basic",
			source: `{ value }`,
			want:   []string{"value"},
		},
		{
			name:   "shorhand",
			source: `{ value, foo, bar, baz, bing }`,
			want:   []string{"value", "foo", "bar", "baz", "bing"},
		},
		{
			name:   "literal",
			source: `{ value: 0 }`,
			want:   []string{"value"},
		},
		{
			name:   "multiple",
			source: `{ a: 0, b: 1, c: 2  }`,
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "objects",
			source: `{ a: { a1: 0 }, b: { b1: { b2: 0 }}, c: { c1: { c2: { c3: 0 }}}  }`,
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "regexp",
			source: `{ a: /hello/g, b: 0 }`,
			want:   []string{"a", "b"},
		},
		{
			name:   "array",
			source: `{ a: [0, 1, 2], b: ["one", "two", "three"], c: 0 }`,
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "valid strings",
			source: `{ "lowercase": true, "camelCase": true, "PascalCase": true, "snake_case": true, "__private": true, ["computed"]: true, }`,
			// Note that quotes are dropped
			want: []string{`lowercase`, `camelCase`, `PascalCase`, `snake_case`, `__private`, `computed`},
		},
		{
			name:   "invalid strings",
			source: `{ "dash-case": true, "with.dot": true, "with space": true }`,
			want:   []string{`"dash-case": dashCase`, `"with.dot": withDot`, `"with space": withSpace`},
		},
	}
	for _, tt := range tests {
		if tt.only {
			tests = make([]keytestcase, 0)
			tests = append(tests, tt)
			break
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := GetObjectKeys([]byte(tt.source))
			output := make([]string, 0)
			for _, key := range keys {
				output = append(output, string(key))
			}
			got, _ := json.Marshal(output)
			want, _ := json.Marshal(tt.want)
			// compare to expected string, show diff if mismatch
			if diff := test_utils.ANSIDiff(string(want), string(got)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
