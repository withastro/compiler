import type * as types from './types.js';
import { compileAstroSync } from '@astrojs/compiler-binding';
import { mapOptions, mapResult } from './shared.js';

type UnwrappedPromise<T> = T extends (...params: any) => Promise<infer Return>
	? (...params: Parameters<T>) => Return
	: T;

export const transform: UnwrappedPromise<typeof types.transform> = (input, options) => {
	const result = mapResult(compileAstroSync(input, mapOptions(options)));

	// Post-process: call resolvePath for each component specifier if provided
	if (typeof options?.resolvePath === 'function') {
		const resolve = options.resolvePath;
		const resolveAll = (components: types.Component[]) => {
			for (const c of components) {
				const resolved = resolve(c.specifier);
				// Sync transform only supports synchronous resolvePath
				if (typeof resolved === 'string') {
					c.resolvedPath = resolved;
				}
			}
		};
		resolveAll(result.hydratedComponents);
		resolveAll(result.clientOnlyComponents);
		resolveAll(result.serverComponents);
	}

	return result;
};

export const parse: UnwrappedPromise<typeof types.parse> = (_input, _options) => {
	throw new Error('parse() is not yet implemented in the Rust compiler');
};

export const convertToTSX: UnwrappedPromise<typeof types.convertToTSX> = (_input, _options) => {
	throw new Error('convertToTSX() is not yet implemented in the Rust compiler');
};
