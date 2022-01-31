import type * as types from '../shared/types';
import { promises as fs } from 'fs';
import Go from './wasm_exec.js';
import { fileURLToPath } from 'url';

export const transform: typeof types.transform = async (input, options) => {
  return getService().then((service) => service.transform(input, options));
};

export const parse: typeof types.parse = async (input, options) => {
  return ensureServiceIsRunning().then((service) => service.parse(input, options));
};

export const compile = async (template: string): Promise<string> => {
  const { default: mod } = await import(`data:text/javascript;charset=utf-8;base64,${Buffer.from(template).toString('base64')}`);
  return mod;
};

interface Service {
  transform: typeof types.transform;
  parse: typeof types.parse;
}

let longLivedService: Promise<Service> | undefined;

let getService = (): Promise<Service> => {
  if (!longLivedService) {
    longLivedService = startRunningService().catch((err) => {
      // Let the caller try again if this fails.
      longLivedService = void 0;
      // But still, throw the error back up the caller.
      throw err;
    });
  }
  return longLivedService;
};

const instantiateWASM = async (wasmURL: string, importObject: Record<string, any>): Promise<WebAssembly.WebAssemblyInstantiatedSource> => {
  let response = undefined;

  const fetchAndInstantiateTask = async () => {
    const wasmArrayBuffer = await fs.readFile(wasmURL).then((res) => res.buffer);
    return WebAssembly.instantiate(new Uint8Array(wasmArrayBuffer), importObject);
  };
  response = await fetchAndInstantiateTask();

  return response;
};

const startRunningService = async (): Promise<Service> => {
  const go = new Go();
  const wasm = await instantiateWASM(fileURLToPath(new URL('../astro.wasm', import.meta.url)), go.importObject);
  go.run(wasm.instance);
  const _service: any = (globalThis as any)['@astrojs/compiler'];
  return {
    transform: (input, options) => new Promise((resolve) => resolve(_service.transform(input, options || {}))),
    parse: (input, options) => new Promise((resolve) => resolve(_service.parse(input, options || {}))).then((result: any) => ({ ...result, ast: JSON.parse(result.ast) })),
  };
};
