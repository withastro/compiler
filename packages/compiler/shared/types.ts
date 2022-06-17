import { RootNode } from './ast';
export * from './ast';

export interface PreprocessorResult {
  code: string;
  map?: string;
}

// eslint-disable-next-line @typescript-eslint/no-empty-interface
export interface ParseOptions {
  position?: boolean;
}

export interface TransformOptions {
  internalURL?: string;
  site?: string;
  sourcefile?: string;
  pathname?: string;
  sourcemap?: boolean | 'inline' | 'external' | 'both';
  /**
   * @deprecated "as" has been removed and no longer has any effect!
   */
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

export interface HydratedComponent {
  exportName: string;
  specifier: string;
  resolvedPath: string;
}

export interface TransformResult {
  css: string[];
  scripts: HoistedScript[];
  hydratedComponents: HydratedComponent[];
  clientOnlyComponents: HydratedComponent[];
  code: string;
  map: string;
}

export interface TSXResult {
  code: string;
  map: string;
}

export interface ParseResult {
  ast: RootNode;
}

// This function transforms a single JavaScript file. It can be used to minify
// JavaScript, convert TypeScript/JSX to JavaScript, or convert newer JavaScript
// to older JavaScript. It returns a promise that is either resolved with a
// "TransformResult" object or rejected with a "TransformFailure" object.
//
// Works in node: yes
// Works in browser: yes
export declare function transform(input: string, options?: TransformOptions): Promise<TransformResult>;

export declare function parse(input: string, options?: ParseOptions): Promise<ParseResult>;

export declare function convertToTSX(input: string, options?: { sourcefile?: string }): Promise<TSXResult>;

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
