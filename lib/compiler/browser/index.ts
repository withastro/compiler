import type * as types from '../shared/types';
import Go from './wasm_exec.js';

export const transform: typeof types.transform = (input, options) => {
  return ensureServiceIsRunning().transform(input, options);
};

interface Service {
  transform: typeof types.transform;
}

let initializePromise: Promise<void> | undefined;
let longLivedService: Service | undefined;

export const initialize: typeof types.initialize = (options) => {
  let wasmURL = options.wasmURL;
  if (!wasmURL) throw new Error('Must provide the "wasmURL" option');
  wasmURL += '';
  if (initializePromise) throw new Error('Cannot call "initialize" more than once');
  initializePromise = startRunningService(wasmURL);
  initializePromise.catch(() => {
    // Let the caller try again if this fails
    initializePromise = void 0;
  });
  return initializePromise;
};

let ensureServiceIsRunning = (): Service => {
  if (longLivedService) return longLivedService;
  if (initializePromise) throw new Error('You need to wait for the promise returned from "initialize" to be resolved before calling this');
  throw new Error('You need to call "initialize" before calling this');
};

const instantiateWASM = async (wasmURL: string, importObject: Record<string, any>): Promise<WebAssembly.WebAssemblyInstantiatedSource> => {
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

const startRunningService = async (wasmURL: string) => {
  const go = new Go();
  const wasm = await instantiateWASM(wasmURL, go.importObject);
  go.run(wasm.instance);

  const apiKeys = new Set(['transform']);
  const service: any = Object.create(null);

  for (const key of apiKeys.values()) {
    const globalKey = `__astro_${key}`;
    service[key] = (globalThis as any)[globalKey];
    delete (globalThis as any)[globalKey];
  }

  longLivedService = {
    transform: (input, options) => new Promise((resolve) => resolve(service.transform(input, options || {}))),
  };
};
