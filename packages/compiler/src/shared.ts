import type { AstroCompileOptions, NapiHoistedScript } from '@astrojs/compiler-binding';
import type { HoistedScript, TransformOptions, TransformResult } from './types.js';
import type { Component } from './types.js';

export function mapOptions(options?: TransformOptions): AstroCompileOptions | undefined {
	if (!options) return undefined;
	return {
		filename: options.filename,
		normalizedFilename: options.normalizedFilename,
		internalUrl: options.internalURL,
		sourcemap: typeof options.sourcemap === 'boolean' ? options.sourcemap : undefined,
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

export function mapResult(result: {
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
}): TransformResult {
	return {
		code: result.code,
		map: result.map,
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
	};
}
