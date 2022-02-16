import type * as types from '../shared/types';
import { promises as fs } from 'fs';
import Go from './wasm_exec.js';
import { fileURLToPath } from 'url';

export const transform: typeof types.transform = async (input, options) => {
  return ensureServiceIsRunning().then((service) => service.transform(input, options));
};

export const compile = async (template: string): Promise<string> => {
  const { default: mod } = await import(`data:text/javascript;charset=utf-8;base64,${Buffer.from(template).toString('base64')}`);
  return mod;
};

interface Service {
  transform: typeof types.transform;
}

let longLivedService: Service | undefined;

let ensureServiceIsRunning = (): Promise<Service> => {
  if (longLivedService) return Promise.resolve(longLivedService);
  return startRunningService();
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

const startRunningService = async () => {
  const go = new Go();
  const wasm = await instantiateWASM(fileURLToPath(new URL('../astro.wasm', import.meta.url)), go.importObject);
  go.run(wasm.instance);

  const service: any = (globalThis as any)['@astrojs/compiler'];

  longLivedService = {
    transform: (input, options) => new Promise((resolve) => resolve(service.transform(input, options || {}))),
  };
  return longLivedService;
};
