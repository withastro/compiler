import type { AstroCompileOptions, NapiHoistedScript, OxcError } from '@astrojs/compiler-binding';
import { extractStylesSync } from '@astrojs/compiler-binding';
import type {
	AsyncTransformOptions,
	HoistedScript,
	ParseResult,
	PreprocessedStyles,
	PreprocessorError,
	PreprocessorResult,
	TransformOptions,
	TransformResult,
} from './types.js';
import type { CompilerError, Component } from './types.js';

export function mapOptions(
	options?: TransformOptions | AsyncTransformOptions,
): AstroCompileOptions | undefined {
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
		internalUrl: options.internalURL,
		sourcemap: options.sourcemap ? true : undefined,
		astroGlobalArgs: options.astroGlobalArgs,
		compact: options.compact,
		resultScopedSlot: options.resultScopedSlot,
		scopedStyleStrategy: options.scopedStyleStrategy,
		transitionsAnimationUrl: options.transitionsAnimationURL,
		annotateSourceFile: options.annotateSourceFile,
		resolvePathProvided: typeof options.resolvePath === 'function' ? true : undefined,
		preprocessedStyles,
	};
}

function mapScript(script: NapiHoistedScript): HoistedScript {
	if (script.type === 'external') {
		return { type: 'external', src: script.src ?? '' };
	}
	return { type: 'inline', code: script.code ?? '', map: '' };
}

export function mapResult(
	result: {
		code: string;
		map: string;
		scope: string;
		css: string[];
		scripts: NapiHoistedScript[];
		hydratedComponents: Component[];
		clientOnlyComponents: Component[];
		serverComponents: Component[];
		containsHead: boolean;
		propagation: boolean;
		styleError: string[];
		errors: OxcError[];
	},
	sourcemapOption?: TransformOptions['sourcemap'],
	preprocessedStyles?: PreprocessedStyles,
): TransformResult {
	let code = result.code;
	const map = result.map;

	// When 'both' or 'inline' sourcemap mode is requested, append an inline
	// sourcemap comment so downstream consumers (e.g. esbuild, Vite module
	// runner) can pick it up directly from the code string.
	if ((sourcemapOption === 'both' || sourcemapOption === 'inline') && map) {
		const base64 = typeof Buffer !== 'undefined' ? Buffer.from(map).toString('base64') : btoa(map);
		code += `\n//# sourceMappingURL=data:application/json;charset=utf-8;base64,${base64}`;
	}

	return {
		code,
		map: sourcemapOption === 'inline' ? '' : map,
		scope: result.scope,
		css: result.css,
		scripts: result.scripts.map(mapScript),
		hydratedComponents: result.hydratedComponents,
		clientOnlyComponents: result.clientOnlyComponents,
		serverComponents: result.serverComponents,
		containsHead: result.containsHead,
		propagation: result.propagation,
		styleError: preprocessedStyles?.styleError ?? result.styleError,
		diagnostics: [],
		errors: result.errors.map(mapError),
	};
}

export function mapParseResult(result: { ast: string; errors: OxcError[] }): ParseResult {
	return {
		ast: JSON.parse(result.ast),
		errors: result.errors.map(mapError),
	};
}

function mapError(error: OxcError): CompilerError {
	return {
		severity: error.severity,
		message: error.message,
		labels: error.labels.map((label) => ({
			message: label.message,
			start: label.start,
			end: label.end,
			line: label.line,
			column: label.column,
		})),
		helpMessage: error.helpMessage,
		codeframe: error.codeframe,
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
