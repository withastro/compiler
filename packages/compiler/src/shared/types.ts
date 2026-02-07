import type { RootNode } from './ast.js';
import type { DiagnosticCode } from './diagnostics.js';
export type * from './ast.js';

export interface PreprocessorResult {
	code: string;
	map?: string;
}

export interface PreprocessorError {
	error: string;
}

export interface ParseOptions {
	position?: boolean;
}

export enum DiagnosticSeverity {
	Error = 1,
	Warning = 2,
	Information = 3,
	Hint = 4,
}

export interface DiagnosticMessage {
	severity: DiagnosticSeverity;
	code: DiagnosticCode;
	location: DiagnosticLocation;
	hint?: string;
	text: string;
}

export interface DiagnosticLocation {
	file: string;
	// 1-based
	line: number;
	// 1-based
	column: number;
	length: number;
}

export interface TransformOptions {
	internalURL?: string;
	filename?: string;
	normalizedFilename?: string;
	sourcemap?: boolean | 'inline' | 'external' | 'both';
	astroGlobalArgs?: string;
	compact?: boolean;
	resultScopedSlot?: boolean;
	scopedStyleStrategy?: 'where' | 'class' | 'attribute';
	/**
	 * @deprecated "as" has been removed and no longer has any effect!
	 */
	as?: 'document' | 'fragment';
	transitionsAnimationURL?: string;
	resolvePath?: (specifier: string) => Promise<string> | string;
	preprocessStyle?: (
		content: string,
		attrs: Record<string, string>
	) => null | Promise<PreprocessorResult | PreprocessorError>;
	annotateSourceFile?: boolean;
	experimentalScriptOrder?: boolean;
}

export type ConvertToTSXOptions = Pick<
	TransformOptions,
	'filename' | 'normalizedFilename' | 'sourcemap'
> & {
	/** If set to true, script tags content will be included in the generated TSX
	 * Scripts will be wrapped in an arrow function to be compatible with JSX's spec
	 */
	includeScripts?: boolean;
	/** If set to true, style tags content will be included in the generated TSX
	 * Styles will be wrapped in a template literal to be compatible with JSX's spec
	 */
	includeStyles?: boolean;
};

export type HoistedScript = { type: string } & (
	| {
			type: 'external';
			src: string;
	  }
	| {
			type: 'inline';
			code: string;
			map: string;
	  }
);

export interface HydratedComponent {
	exportName: string;
	localName: string;
	specifier: string;
	resolvedPath: string;
}

export interface TransformResult {
	code: string;
	map: string;
	scope: string;
	styleError: string[];
	diagnostics: DiagnosticMessage[];
	css: string[];
	scripts: HoistedScript[];
	hydratedComponents: HydratedComponent[];
	clientOnlyComponents: HydratedComponent[];
	serverComponents: HydratedComponent[];
	containsHead: boolean;
	propagation: boolean;
}

export interface SourceMap {
	file: string;
	mappings: string;
	names: string[];
	sources: string[];
	sourcesContent: string[];
	version: number;
}

/**
 * Represents a location in a TSX file.
 * Both the `start` and `end` properties are 0-based, and are based off utf-16 code units. (i.e. JavaScript's `String.prototype.length`)
 */
export interface TSXLocation {
	start: number;
	end: number;
}

export interface TSXExtractedTag {
	position: TSXLocation;
	content: string;
}

export interface TSXExtractedScript extends TSXExtractedTag {
	type: 'processed-module' | 'module' | 'inline' | 'event-attribute' | 'json' | 'raw' | 'unknown';
}

export interface TSXExtractedStyle extends TSXExtractedTag {
	type: 'tag' | 'style-attribute';
	lang:
		| 'css'
		| 'scss'
		| 'sass'
		| 'less'
		| 'stylus'
		| 'styl'
		| 'postcss'
		| 'pcss'
		| 'unknown'
		| (string & {});
}

export interface TSXResult {
	code: string;
	map: SourceMap;
	diagnostics: DiagnosticMessage[];
	metaRanges: {
		frontmatter: TSXLocation;
		body: TSXLocation;
		scripts?: TSXExtractedScript[];
		styles?: TSXExtractedStyle[];
	};
}

export interface ParseResult {
	ast: RootNode;
	diagnostics: DiagnosticMessage[];
}

export declare function transform(
	input: string,
	options?: TransformOptions
): Promise<TransformResult>;

export declare function parse(input: string, options?: ParseOptions): Promise<ParseResult>;

export declare function convertToTSX(
	input: string,
	options?: ConvertToTSXOptions
): Promise<TSXResult>;

/**
 * No-op. Exists for backwards compatibility with the WASM-based compiler.
 */
export declare function teardown(): void;
