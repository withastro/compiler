import './lib.deno.ns';
import type * as types from "../shared/types";
import Go from "./wasm_exec";

export const transform: typeof types.transform = async (input, options) => {
  return ensureServiceIsRunning().then(service => service.transform(input, options));
};

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
  let response = undefined;

  const fetchAndInstantiateTask = async () => {
      const wasmArrayBuffer = await fetch(wasmURL).then((response) => response.arrayBuffer());
      return WebAssembly.instantiate(new Uint8Array(wasmArrayBuffer), importObject);
  };
  response = await fetchAndInstantiateTask();

  return response;
};

const startRunningService = async () => {
  const go = new Go();
  const wasm = await instantiateWASM(new URL('../astro.wasm', import.meta.url).toString(), go.importObject);
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
