export type { PreprocessorResult, ParseOptions, TransformOptions, HoistedScript, TransformResult, ParseResult } from '../shared/types';
import type * as types from '../shared/types';
import { promises as fs } from 'fs';
import Go from './wasm_exec.js';
import { fileURLToPath } from 'url';

export async function initialize() {
  await getService();
}

export const transform: typeof types.transform = async (input, options) => {
  return getService().then((service) => service.transform(input, options));
};

export const parse: typeof types.parse = async (input, options) => {
  return getService().then((service) => service.parse(input, options));
};

export const parseSync: typeof types.parseSync = (input, options) => {
  if (!service) {
    throw new Error(`You need to call "initialize" before calling "parseSync"`);
  }
  return service.parseSync(input, options);
};

export const convertToTSX: typeof types.convertToTSX = async (input, options) => {
  return getService().then((service) => service.convertToTSX(input, options));
};

export const compile = async (template: string): Promise<string> => {
  const { default: mod } = await import(`data:text/javascript;charset=utf-8;base64,${Buffer.from(template).toString('base64')}`);
  return mod;
};

interface Service {
  transform: typeof types.transform;
  parse: typeof types.parse;
  parseSync: typeof types.parseSync;
  convertToTSX: typeof types.convertToTSX;
}

let longLivedService: Promise<Service> | undefined;
let service: Service | undefined;

let getService = async (): Promise<Service> => {
  if (!longLivedService) {
    longLivedService = startRunningService()
      .then((s) => {
        service = s;
        return s;
      })
      .catch((err) => {
        // Let the caller try again if this fails.
        longLivedService = void 0;
        // But still, throw the error back up the caller.
        throw err;
      });
  }
  service = await longLivedService;
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
    transform: (input, options) =>
      new Promise((resolve) => {
        try {
          resolve(_service.transform(input, options || {}));
        } catch (err) {
          // Recreate the service next time on panic
          longLivedService = void 0;
          throw err;
        }
      }),
    parse: (input, options) => new Promise((resolve) => resolve(_service.parse(input, options || {}))).then((result: any) => ({ ...result, ast: JSON.parse(result.ast) })),
    parseSync: (input, options) => {
      const result = _service.parseSync(input, options || {});
      return { ...result, ast: JSON.parse(result.ast) };
    },
    convertToTSX: (input, options) => {
      return new Promise((resolve) => resolve(_service.convertToTSX(input, options || {}))).then((result: any) => {
        return { ...result, map: JSON.parse(result.map) };
      });
    },
  };
};
