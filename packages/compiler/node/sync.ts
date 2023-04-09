import { promises as fs } from 'fs';
import type * as types from '../shared/types';
import Go from './wasm_exec';

type UnwrappedPromise<T> = T extends (...params: any) => Promise<infer Return> ? (...params: Parameters<T>) => Return : T;

interface Service {
  transform: UnwrappedPromise<typeof types.transform>;
  parse: UnwrappedPromise<typeof types.parse>;
  convertToTSX: UnwrappedPromise<typeof types.convertToTSX>;
}

function getService(): Service {
  if (!longLivedService) {
    throw new Error("Service hasn't been started. Start it with startRunningService.");
  }
  return longLivedService;
}

let longLivedService: Service | undefined;

export const transform = ((input, options) => getService().transform(input, options)) satisfies Service['transform'];

export const parse = ((input, options) => {
  return getService().parse(input, options);
}) satisfies Service['parse'];

export const convertToTSX = ((input, options) => {
  return getService().convertToTSX(input, options);
}) satisfies Service['convertToTSX'];

export async function startRunningService(wasmPath: string) {
  const go = new Go();
  const wasm = await instantiateWASM(wasmPath, go.importObject);
  go.run(wasm.instance);
  const _service: any = (globalThis as any)['@astrojs/compiler'];
  longLivedService = {
    transform: (input, options) => {
      try {
        return _service.transform(input, options || {});
      } catch (err) {
        // Recreate the service next time on panic
        longLivedService = void 0;
        throw err;
      }
    },
    parse: (input, options) => {
      const result = _service.parse(input, options || {});
      return { ...result, ast: JSON.parse(result.ast) };
    },
    convertToTSX: (input, options) => {
      const result = _service.convertToTSX(input, options || {});
      return { ...result, map: JSON.parse(result.map) };
    },
  };

  return 'hey';
}

async function instantiateWASM(wasmURL: string, importObject: Record<string, any>): Promise<WebAssembly.WebAssemblyInstantiatedSource> {
  let response = undefined;

  const fetchAndInstantiateTask = async () => {
    const wasmArrayBuffer = await fs.readFile(wasmURL).then((res) => res.buffer);
    return WebAssembly.instantiate(new Uint8Array(wasmArrayBuffer), importObject);
  };
  response = await fetchAndInstantiateTask();

  return response;
}
