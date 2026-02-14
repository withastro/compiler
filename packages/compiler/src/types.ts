// Re-export types that are identical to the NAPI binding.
// The Rust codegen crate is the source of truth for these types.
export type {
	CompilerError,
	CompilerErrorLabel,
	CompileResult,
	Component,
	HoistedScript,
	ParseResult as BindingParseResult,
	StyleBlock,
} from '@astrojs/compiler-binding';
import type { CompileOptions, CompileResult } from '@astrojs/compiler-binding';

// ---- TS-only types (not expressible in the NAPI layer) ----

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

/**
 * Options for compiling Astro files to JavaScript.
 *
 * Extends the NAPI `CompileOptions` with TS-only features:
 * - `resolvePath` callback for post-compilation path resolution
 * - `preprocessedStyles` uses the opaque `PreprocessedStyles` type
 *
 * Fields that are internal to the NAPI layer (`resolvePathProvided`,
 * `stripSlotComments`) are omitted — they are set automatically by
 * the wrapper functions.
 */
export interface TransformOptions
	extends Omit<CompileOptions, 'resolvePathProvided' | 'preprocessedStyles'> {
	resolvePath?: (specifier: string) => string;
	/**
	 * Preprocessed style content from {@link preprocessStyles}.
	 *
	 * When provided, the compiler uses these preprocessed CSS strings
	 * instead of the raw `<style>` content from the template.
	 */
	preprocessedStyles?: PreprocessedStyles;
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

/** The public result type returned by `transform()` / async `transform()`. */
export type TransformResult = CompileResult;

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
	errors: import('@astrojs/compiler-binding').CompilerError[];
}

// TODO: Stub until TSX is implemented in the Rust compiler
export type TSXResult = any;
