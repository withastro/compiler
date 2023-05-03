import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import type * as types from '../shared/types';
import Go from './wasm_exec.js';

type UnwrappedPromise<T> = T extends (...params: any) => Promise<infer Return> ? (...params: Parameters<T>) => Return : T;

interface Service {
  transform: UnwrappedPromise<typeof types.transform>;
  parse: UnwrappedPromise<typeof types.parse>;
  convertToTSX: UnwrappedPromise<typeof types.convertToTSX>;
}

function getService(): Service {
  if (!longLivedService) {
    longLivedService = startRunningService();
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

export function startRunningService(): Service {
  const go = new Go();
  const wasm = instantiateWASM(join(__dirname, '../astro.wasm'), go.importObject);
  go.run(wasm);
  const _service: any = (globalThis as any)['@astrojs/compiler'];
  return {
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
      try {
        const result = _service.parse(input, options || {});
        return { ...result, ast: JSON.parse(result.ast) };
      } catch (err) {
        longLivedService = void 0;
        throw err;
      }
    },
    convertToTSX: (input, options) => {
      try {
        const result = _service.convertToTSX(input, options || {});
        return { ...result, map: JSON.parse(result.map) };
      } catch (err) {
        longLivedService = void 0;
        throw err;
      }
    },
  };
}

function instantiateWASM(wasmURL: string, importObject: Record<string, any>): WebAssembly.Instance {
  const wasmArrayBuffer = readFileSync(wasmURL);
  return new WebAssembly.Instance(new WebAssembly.Module(wasmArrayBuffer), importObject);
}
