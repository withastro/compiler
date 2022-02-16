import type * as types from './types.ts';
import './wasm_exec.js';

const Go = (globalThis as any).Go;

export const transform: typeof types.transform = async (input, options) => {
  const service = await ensureServiceIsRunning();
  return await service.transform(input, {
    internalURL: new URL('./internal.ts', import.meta.url).toString(),
    ...options,
  });
};

export const compile: typeof types.compile = async (transformResult: types.TransformResult): Promise<string> => {
  const { renderPage } = await import(new URL('./internal.ts', import.meta.url).toString());

  const result = {
    styles: new Set(),
    scripts: new Set(),
    /** This function returns the `Astro` faux-global */
    createAstro: (props: any) => {
      return {
        isPage: true,
        site: null,
        request: { url: null, canonicalURL: null },
        props,
        fetchContent: () => {},
      };
    },
  };

  const { default: Component } = await import(`data:text/typescript;charset=utf-8;base64,${btoa(transformResult.code)}`);
  let html = await renderPage(result, Component, {}, {});
  return html;
};

interface Service {
  transform: typeof types.transform;
}

let longLivedService: Service | undefined;

const ensureServiceIsRunning = (): Promise<Service> => {
  if (longLivedService) return Promise.resolve(longLivedService);
  return startRunningService();
};

const instantiateWASM = async (wasmURL: string, importObject: Record<string, any>): Promise<WebAssembly.WebAssemblyInstantiatedSource> => {
  if (wasmURL.startsWith('file://')) {
    const bytes = await Deno.readFile('./astro.wasm');
    return await WebAssembly.instantiate(bytes, importObject);
  } else {
    return await WebAssembly.instantiateStreaming(fetch(wasmURL), importObject);
  }
};

const startRunningService = async () => {
  const go = new Go();
  const wasm = await instantiateWASM(new URL('./astro.wasm', import.meta.url).toString(), go.importObject);
  go.run(wasm.instance);

  const service: any = (globalThis as any)['@astrojs/compiler'];

  longLivedService = {
    transform: (input, options) => new Promise((resolve) => resolve(service.transform(input, options || {}))),
  };
  return longLivedService;
};
