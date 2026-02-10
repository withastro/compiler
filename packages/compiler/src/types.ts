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

export interface TransformOptions {
	internalURL?: string;
	filename?: string;
	normalizedFilename?: string;
	sourcemap?: boolean | 'inline' | 'external' | 'both';
	astroGlobalArgs?: string;
	compact?: boolean;
	resultScopedSlot?: boolean;
	scopedStyleStrategy?: 'where' | 'class' | 'attribute';
	transitionsAnimationURL?: string;
	resolvePath?: (specifier: string) => Promise<string> | string;
	preprocessStyle?: (
		content: string,
		attrs: Record<string, string>,
	) => null | Promise<PreprocessorResult | PreprocessorError>;
	annotateSourceFile?: boolean;
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

export interface Component {
	exportName: string;
	localName: string;
	specifier: string;
	resolvedPath: string;
}

export interface CompilerErrorLabel {
	message: string | null;
	/** Byte offset start in source */
	start: number;
	/** Byte offset end in source */
	end: number;
	/** 1-based line number in the source */
	line: number;
	/** 0-based column number in the source */
	column: number;
}

export interface CompilerError {
	severity: 'Error' | 'Warning' | 'Advice';
	message: string;
	labels: CompilerErrorLabel[];
	helpMessage: string | null;
	codeframe: string | null;
}

export interface TransformResult {
	code: string;
	map: string;
	scope: string;
	styleError: string[];
	// TODO: Currently always empty on the Rust compiler
	diagnostics: any[];
	/** Compilation errors from the Rust compiler (oxc-based). */
	errors: CompilerError[];
	css: string[];
	scripts: HoistedScript[];
	hydratedComponents: Component[];
	clientOnlyComponents: Component[];
	serverComponents: Component[];
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

// TODO: Stub until these are implemented in the Rust compiler
export type ParseResult = any;
export type TSXResult = any;

export declare function transform(
	input: string,
	options?: TransformOptions,
): Promise<TransformResult>;

export declare function parse(input: string, options?: ParseOptions): Promise<ParseResult>;

export declare function convertToTSX(
	input: string,
	options?: ConvertToTSXOptions,
): Promise<TSXResult>;
