package js_scanner

import (
	"bytes"
	"strings"
	"testing"

	"github.com/withastro/compiler/internal/test_utils"
)

type testcase struct {
	name   string
	source string
	want   string
	only   bool
}

func TestHoistImport(t *testing.T) {
	tests := []testcase{
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
			want: `import { fn } from "package";`,
		},
		{
			name: "import assertion",
			source: `// comment
import { fn } from "package" assert { it: 'works' };
const b = await fetch();`,
			want: `import { fn } from "package" assert { it: 'works' };`,
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
			want: `import {
  fn
} from
  "package" assert {
    it: 'works'
  };
`,
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
			want: `import { b } from "b";`,
		},
		{
			name: "getStaticPaths with regex and following content",
			source: `export async function getStaticPaths() {
  const value = /2/g;
}
// comment
import { b } from "b";
const { a } = Astro.props;`,
			want: `import { b } from "b";`,
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
import { c } from "c";
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
	for _, tt := range tests {
		if tt.only {
			tests = make([]testcase, 0)
			tests = append(tests, tt)
			break
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HoistImports([]byte(tt.source))
			got := []byte{}
			for _, imp := range result.Hoisted {
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
			want: `export async function getStaticPaths() {
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
			want: `export async function getStaticPaths() {
  const value = /2/g;
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
			want: `export interface Props {}
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
export B from "./_types"
export type C from "./_types"`,
			want: `export * from "./local-data.json";
export { default as A } from "./_types"
export B from "./_types"
export type C from "./_types"`,
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
			want: `export interface LocalImageProps extends Omit<TransformOptions, 'src'>, Omit<ImageAttributes, 'src' | 'width' | 'height'> {
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
			result := HoistExports([]byte(tt.source))
			got := []byte{}
			for _, imp := range result.Hoisted {
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
