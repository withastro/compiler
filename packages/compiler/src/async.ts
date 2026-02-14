export type {
	CompilerError,
	CompilerErrorLabel,
	CompileResult,
	HoistedScript,
	ParseOptions,
	ParseResult,
	PreprocessedStyles,
	PreprocessorError,
	PreprocessorResult,
	TransformResult,
} from './types.js';
export type { AsyncTransformOptions as TransformOptions } from './types.js';
import { compileAstro, parseAstro } from '@astrojs/compiler-binding';
import { mapOptions, mapParseResult, mapResult } from './shared.js';
export { preprocessStyles } from './shared.js';
import type { AsyncTransformOptions, Component, ParseResult, TransformResult } from './types.js';

export async function transform(
	input: string,
	options?: AsyncTransformOptions,
): Promise<TransformResult> {
	const result = mapResult(
		await compileAstro(input, mapOptions(options)),
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
			c.resolvedPath = await resolve(c.specifier);
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

export async function parse(input: string): Promise<ParseResult> {
	return mapParseResult(await parseAstro(input));
}

export async function convertToTSX(_input: string): Promise<never> {
	throw new Error('convertToTSX() is not yet implemented in the Rust compiler');
}
