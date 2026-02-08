export type {
	HoistedScript,
	ParseOptions,
	ParseResult,
	PreprocessorResult,
	TransformOptions,
	TransformResult,
} from './types.js';
import type * as types from './types.js';
import { compileAstro } from '@astrojs/compiler-binding';
import { mapOptions, mapResult } from './shared.js';

export const transform: typeof types.transform = async (input, options) => {
	const result = mapResult(await compileAstro(input, mapOptions(options)));

	// Post-process: call resolvePath for each component specifier if provided
	if (typeof options?.resolvePath === 'function') {
		const resolve = options.resolvePath;
		const resolveAll = async (components: types.Component[]) => {
			for (const c of components) {
				c.resolvedPath = await resolve(c.specifier);
			}
		};
		await resolveAll(result.hydratedComponents);
		await resolveAll(result.clientOnlyComponents);
		await resolveAll(result.serverComponents);
	}

	return result;
};

export const parse: typeof types.parse = async (_input, _options) => {
	throw new Error('parse() is not yet implemented in the Rust compiler');
};

export const convertToTSX: typeof types.convertToTSX = async (_input, _options) => {
	throw new Error('convertToTSX() is not yet implemented in the Rust compiler');
};
