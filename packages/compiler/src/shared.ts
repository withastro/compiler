import type { CompileOptions, CompileResult } from '@astrojs/compiler-binding';
import { extractStylesSync } from '@astrojs/compiler-binding';
import type {
	AsyncTransformOptions,
	ParseResult,
	PreprocessedStyles,
	PreprocessorError,
	PreprocessorResult,
	TransformOptions,
	TransformResult,
} from './types.js';

/**
 * Map public `TransformOptions` to the NAPI `CompileOptions`.
 *
 * Handles:
 * - `resolvePath` callback → `resolvePathProvided` flag
 * - `preprocessedStyles` opaque type → raw array
 */
export function mapOptions(
	options?: TransformOptions | AsyncTransformOptions,
): CompileOptions | undefined {
	if (!options) return undefined;

	// Map PreprocessedStyles to the NAPI format
	let preprocessedStyles: (string | undefined)[] | undefined;
	if (options.preprocessedStyles) {
		const hasAny = options.preprocessedStyles.styles.some((s) => s != null);
		if (hasAny) {
			preprocessedStyles = options.preprocessedStyles.styles;
		}
	}

	return {
		filename: options.filename,
		normalizedFilename: options.normalizedFilename,
		internalURL: options.internalURL,
		sourcemap: options.sourcemap,
		astroGlobalArgs: options.astroGlobalArgs,
		compact: options.compact,
		resultScopedSlot: options.resultScopedSlot,
		scopedStyleStrategy: options.scopedStyleStrategy,
		transitionsAnimationURL: options.transitionsAnimationURL,
		annotateSourceFile: options.annotateSourceFile,
		resolvePathProvided: typeof options.resolvePath === 'function' ? true : undefined,
		preprocessedStyles,
	};
}

/**
 * Map the NAPI `CompileResult` to the public `TransformResult`.
 *
 * Merges preprocessed style errors into the result.
 */
export function mapResult(
	result: CompileResult,
	preprocessedStyles?: PreprocessedStyles,
): TransformResult {
	if (preprocessedStyles?.styleError?.length) {
		return {
			...result,
			styleError: preprocessedStyles.styleError,
		};
	}
	return result;
}

/**
 * Map the NAPI `ParseResult` to the public `ParseResult`.
 *
 * The only transformation is parsing the AST JSON string into an object.
 */
export function mapParseResult(result: {
	ast: string;
	diagnostics: CompileResult['diagnostics'];
}): ParseResult {
	return {
		ast: JSON.parse(result.ast),
		diagnostics: result.diagnostics,
	};
}

// --- Style preprocessing ---

function isPreprocessorError(
	result: PreprocessorResult | PreprocessorError,
): result is PreprocessorError {
	return 'error' in result && typeof (result as PreprocessorError).error === 'string';
}

/** Collect raw callback results into a {@link PreprocessedStyles} object. */
function collectResults(
	results: (PreprocessorResult | PreprocessorError | null)[],
): PreprocessedStyles {
	const styleError: string[] = [];
	const styles: (string | undefined)[] = results.map((result) => {
		if (result == null) return undefined;
		if (isPreprocessorError(result)) {
			styleError.push(result.error);
			return ''; // Empty content on error
		}
		return result.code;
	});
	return { styles, styleError };
}

/** Sync callback signature for {@link preprocessStyles}. */
type SyncPreprocessStyleFn = (
	content: string,
	attrs: Record<string, string>,
) => null | PreprocessorResult | PreprocessorError;

/** Async callback signature for {@link preprocessStyles}. */
type AsyncPreprocessStyleFn = (
	content: string,
	attrs: Record<string, string>,
) => Promise<PreprocessorResult | PreprocessorError | null>;

/**
 * Extract and preprocess `<style>` blocks from an Astro source string.
 *
 * Call this **before** `transform()` and pass the result via
 * `options.preprocessedStyles`.
 *
 * The return type mirrors the callback: if `preprocessStyle` is sync,
 * this returns `PreprocessedStyles` synchronously.
 * If the callback is async, this returns `Promise<PreprocessedStyles>`.
 */
export function preprocessStyles(
	input: string,
	preprocessStyle: SyncPreprocessStyleFn,
): PreprocessedStyles;
export function preprocessStyles(
	input: string,
	preprocessStyle: AsyncPreprocessStyleFn,
): Promise<PreprocessedStyles>;
export function preprocessStyles(
	input: string,
	preprocessStyle: SyncPreprocessStyleFn | AsyncPreprocessStyleFn,
): PreprocessedStyles | Promise<PreprocessedStyles> {
	const blocks = extractStylesSync(input);

	if (blocks.length === 0) {
		return { styles: [], styleError: [] };
	}

	// Call the preprocessor for each block, collecting results.
	// We detect sync vs async by checking if any result is a Promise.
	const results: (
		| PreprocessorResult
		| PreprocessorError
		| null
		| Promise<PreprocessorResult | PreprocessorError | null>
	)[] = [];
	let hasPromise = false;

	for (const block of blocks) {
		try {
			const result = preprocessStyle(block.content, block.attrs);
			if (result != null && typeof (result as Promise<unknown>).then === 'function') {
				hasPromise = true;
			}
			results.push(result);
		} catch (err) {
			results.push({ error: err instanceof Error ? err.message : String(err) });
		}
	}

	if (!hasPromise) {
		// All results are synchronous — return synchronously
		return collectResults(results as (PreprocessorResult | PreprocessorError | null)[]);
	}

	// At least one result is a Promise — await all of them
	return Promise.all(
		results.map(async (r) => {
			try {
				return await r;
			} catch (err) {
				return { error: err instanceof Error ? err.message : String(err) } as PreprocessorError;
			}
		}),
	).then(collectResults);
}
