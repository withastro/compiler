export interface PreprocessorResult {
	code: string;
	map?: string;
}

export interface PreprocessorError {
	error: string;
}

/**
 * Result of preprocessing `<style>` blocks via {@link preprocessStyles}.
 *
 * This is an opaque value — pass it to `transform()` via the
 * `preprocessedStyles` option. Do not inspect or modify it.
 */
export interface PreprocessedStyles {
	/** Preprocessed CSS per extractable `<style>`, in document order.
	 *  `undefined` = use original content, `""` = error (empty). */
	styles: (string | undefined)[];
	/** Error messages from the preprocessor. */
	styleError: string[];
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
	resolvePath?: (specifier: string) => string;
	/**
	 * Preprocessed style content from {@link preprocessStyles}.
	 *
	 * When provided, the compiler uses these preprocessed CSS strings
	 * instead of the raw `<style>` content from the template.
	 */
	preprocessedStyles?: PreprocessedStyles;
	annotateSourceFile?: boolean;
}

/** TransformOptions variant for the async entrypoint, where resolvePath may return a Promise. */
export interface AsyncTransformOptions extends Omit<TransformOptions, 'resolvePath'> {
	resolvePath?: (specifier: string) => Promise<string> | string;
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

/** Result of parsing an Astro file into an AST. */
export interface ParseResult {
	/** The oxc AST in ESTree-compatible JSON format. */
	ast: Record<string, any>;
	/** Parse errors encountered. */
	errors: CompilerError[];
}

// TODO: Stub until TSX is implemented in the Rust compiler
export type TSXResult = any;

export declare function preprocessStyles(
	input: string,
	preprocessStyle: (
		content: string,
		attrs: Record<string, string>,
	) => null | PreprocessorResult | PreprocessorError,
): PreprocessedStyles;
export declare function preprocessStyles(
	input: string,
	preprocessStyle: (
		content: string,
		attrs: Record<string, string>,
	) => Promise<PreprocessorResult | PreprocessorError | null>,
): Promise<PreprocessedStyles>;

export declare function transform(input: string, options?: TransformOptions): TransformResult;

export declare function parse(input: string, options?: ParseOptions): ParseResult;

export declare function convertToTSX(input: string, options?: ConvertToTSXOptions): TSXResult;
