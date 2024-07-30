import type * as types from '../shared/types.js';
import Go from './wasm_exec.js';

export const transform: typeof types.transform = (input, options) => {
	return ensureServiceIsRunning().transform(input, options);
};

export const parse: typeof types.parse = (input, options) => {
	return ensureServiceIsRunning().parse(input, options);
};

export const convertToTSX: typeof types.convertToTSX = (input, options) => {
	return ensureServiceIsRunning().convertToTSX(input, options);
};

interface Service {
	transform: typeof types.transform;
	parse: typeof types.parse;
	convertToTSX: typeof types.convertToTSX;
}

let initializePromise: Promise<Service> | undefined;
let longLivedService: Service | undefined;

export const teardown: typeof types.teardown = () => {
	initializePromise = undefined;
	longLivedService = undefined;
	(globalThis as any)['@astrojs/compiler'] = undefined;
};

export const initialize: typeof types.initialize = async (options) => {
	let wasmURL = options.wasmURL;
	if (!wasmURL) throw new Error('Must provide the "wasmURL" option');
	wasmURL += '';
	if (!initializePromise) {
		initializePromise = startRunningService(wasmURL).catch((err) => {
			// Let the caller try again if this fails.
			initializePromise = void 0;
			// But still, throw the error back up the caller.
			throw err;
		});
	}
	longLivedService = longLivedService || (await initializePromise);
};

const ensureServiceIsRunning = (): Service => {
	if (!initializePromise) throw new Error('You need to call "initialize" before calling this');
	if (!longLivedService)
		throw new Error(
			'You need to wait for the promise returned from "initialize" to be resolved before calling this'
		);
	return longLivedService;
};

const instantiateWASM = async (
	wasmURL: string,
	importObject: Record<string, any>
): Promise<WebAssembly.WebAssemblyInstantiatedSource> => {
	let response = undefined;

	if (WebAssembly.instantiateStreaming) {
		response = await WebAssembly.instantiateStreaming(fetch(wasmURL), importObject);
	} else {
		const fetchAndInstantiateTask = async () => {
			const wasmArrayBuffer = await fetch(wasmURL).then((res) => res.arrayBuffer());
			return WebAssembly.instantiate(wasmArrayBuffer, importObject);
		};
		response = await fetchAndInstantiateTask();
	}

	return response;
};

const startRunningService = async (wasmURL: string): Promise<Service> => {
	const go = new Go();
	const wasm = await instantiateWASM(wasmURL, go.importObject);
	go.run(wasm.instance);

	const service: any = (globalThis as any)['@astrojs/compiler'];

	return {
		transform: (input, options) =>
			new Promise((resolve) => resolve(service.transform(input, options || {}))),
		convertToTSX: (input, options) =>
			new Promise((resolve) => resolve(service.convertToTSX(input, options || {}))).then(
				(result: any) => ({
					...result,
					map: JSON.parse(result.map),
				})
			),
		parse: (input, options) =>
			new Promise((resolve) => resolve(service.parse(input, options || {}))).then(
				(result: any) => ({ ...result, ast: JSON.parse(result.ast) })
			),
	};
};
