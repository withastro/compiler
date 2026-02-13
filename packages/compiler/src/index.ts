export type {
	CompilerError,
	CompilerErrorLabel,
	HoistedScript,
	ParseOptions,
	ParseResult,
	PreprocessedStyles,
	PreprocessorError,
	PreprocessorResult,
	TransformOptions,
	TransformResult,
} from './types.js';
import { compileAstroSync, parseAstroSync } from '@astrojs/compiler-binding';
import { mapOptions, mapParseResult, mapResult } from './shared.js';
export { preprocessStyles } from './shared.js';
import type {
	Component,
	ConvertToTSXOptions,
	ParseResult,
	TransformOptions,
	TransformResult,
} from './types.js';

export function transform(input: string, options?: TransformOptions): TransformResult {
	const result = mapResult(
		compileAstroSync(input, mapOptions(options)),
		options?.sourcemap,
		options?.preprocessedStyles,
	);

	// Post-process: call resolvePath for each component specifier if provided.
	if (typeof options?.resolvePath === 'function') {
		const resolve = options.resolvePath;
		const allComponents: Component[] = [
			...result.hydratedComponents,
			...result.clientOnlyComponents,
			...result.serverComponents,
		];
		for (const c of allComponents) {
			c.resolvedPath = resolve(c.specifier);
		}

		// Rewrite client:component-path values in the generated code
		let { code } = result;
		for (const c of allComponents) {
			if (c.resolvedPath && c.resolvedPath !== c.specifier) {
				code = code
					.split(`"client:component-path": "${c.specifier}"`)
					.join(`"client:component-path": "${c.resolvedPath}"`);
			}
		}
		result.code = code;
	}

	return result;
}

export function parse(input: string): ParseResult {
	return mapParseResult(parseAstroSync(input));
}

export function convertToTSX(_input: string, _options: ConvertToTSXOptions): never {
	throw new Error('convertToTSX() is not yet implemented');
}
