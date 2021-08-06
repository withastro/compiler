import type * as types from "./types.ts";
import "./wasm_exec.js";

const Go = (globalThis as any).Go;

export const transform: typeof types.transform = async (input, options) => {
  return ensureServiceIsRunning().then(service => service.transform(input, { internalURL: new URL('./shim.ts', import.meta.url).toString(), ...options }));
};

export const compile = async (template: string): Promise<string> => {
  const { default: mod } = await import(`data:text/typescript;charset=utf-8;base64,${btoa(template)}`)
  return mod.__render()
}

interface Service {
  transform: typeof types.transform;
}

let longLivedService: Service | undefined;

let ensureServiceIsRunning = (): Promise<Service> => {
  if (longLivedService) return Promise.resolve(longLivedService);
  return startRunningService();
}

const instantiateWASM = async (
  wasmURL: string,
  importObject: Record<string, any>
): Promise<WebAssembly.WebAssemblyInstantiatedSource> => {
  return await WebAssembly.instantiateStreaming(
    fetch(wasmURL),
    importObject
  );
};

const startRunningService = async () => {
  const go = new Go();
  const wasm = await instantiateWASM(new URL('./astro.wasm', import.meta.url).toString(), go.importObject);
  go.run(wasm.instance);

  const apiKeys = new Set([
    'transform'
  ]);
  const service: any = Object.create(null);

  for (const key of apiKeys.values()) {
    const globalKey = `__astro_${key}`;
    service[key] = (globalThis as any)[globalKey];
    delete (globalThis as any)[globalKey];
  }

  longLivedService = {
    transform: (input, options) => new Promise((resolve) => resolve(service.transform(input, options || {})))
  };
  return longLivedService;
};
