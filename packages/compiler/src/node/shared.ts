import type * as types from '../shared/types.js';
import type { AstroCompileOptions, NapiHoistedScript } from '@astrojs/compiler-binding';

export function mapOptions(options?: types.TransformOptions): AstroCompileOptions | undefined {
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
		experimentalScriptOrder: options.experimentalScriptOrder,
		resolvePathProvided: typeof options.resolvePath === 'function' ? true : undefined,
	};
}

function mapScript(script: NapiHoistedScript): types.HoistedScript {
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
	hydratedComponents: types.HydratedComponent[];
	clientOnlyComponents: types.HydratedComponent[];
	serverComponents: types.HydratedComponent[];
	containsHead: boolean;
	propagation: boolean;
	styleError: string[];
}): types.TransformResult {
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
