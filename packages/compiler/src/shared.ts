import type { AstroCompileOptions, NapiHoistedScript, OxcError } from '@astrojs/compiler-binding';
import type {
	AsyncTransformOptions,
	HoistedScript,
	ParseResult,
	TransformOptions,
	TransformResult,
} from './types.js';
import type { CompilerError, Component } from './types.js';

export function mapOptions(
	options?: TransformOptions | AsyncTransformOptions
): AstroCompileOptions | undefined {
	if (!options) return undefined;
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
	sourcemapOption?: TransformOptions['sourcemap']
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
		styleError: result.styleError,
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
