export interface PreprocessorResult {
  code: string;
  map?: string;
}

export interface TransformOptions {
  internalURL?: string;
  site?: string;
  sourcefile?: string;
  pathname?: string;
  sourcemap?: boolean | 'inline' | 'external' | 'both';
  as?: 'document' | 'fragment';
  projectRoot?: string;
  preprocessStyle?: (content: string, attrs: Record<string, string>) => Promise<PreprocessorResult>;
  experimentalStaticExtraction?: boolean;
}

export type HoistedScript = { type: string } & (
  | {
      type: 'external';
      src: string;
    }
  | {
      type: 'inline';
      code: string;
    }
);

export interface TransformResult {
  css: string[];
  scripts: HoistedScript[];
  code: string;
  map: string;
}

// This function transforms a single JavaScript file. It can be used to minify
// JavaScript, convert TypeScript/JSX to JavaScript, or convert newer JavaScript
// to older JavaScript. It returns a promise that is either resolved with a
// "TransformResult" object or rejected with a "TransformFailure" object.
//
// Works in node: yes
// Works in browser: yes
export declare function transform(input: string, options?: TransformOptions): Promise<TransformResult>;

// This configures the browser-based version of astro. It is necessary to
// call this first and wait for the returned promise to be resolved before
// making other API calls when using astro in the browser.
//
// Works in node: yes
// Works in browser: yes ("options" is required)
export declare function initialize(options: InitializeOptions): Promise<void>;

export interface InitializeOptions {
  // The URL of the "astro.wasm" file. This must be provided when running
  // astro in the browser.
  wasmURL?: string;
}
