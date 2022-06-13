import { RootNode } from './ast';
export * from './ast';

export interface PreprocessorResult {
  code: string;
  map?: string;
}

export interface PreprocessorError {
  error: string;
}

// eslint-disable-next-line @typescript-eslint/no-empty-interface
export interface ParseOptions {
  position?: boolean;
}

export interface Message {
  location: MessageLocation;
  text: string;
}

export interface MessageLocation {
  file: string;
  lineText?: string;
  suggestion?: string;
  // 1-based
  line: number;
  // 0-based, in bytes
  column: number;
  // in bytes
  length: number;
}

export interface TransformOptions {
  internalURL?: string;
  site?: string;
  sourcefile?: string;
  pathname?: string;
  moduleId?: string;
  sourcemap?: boolean | 'inline' | 'external' | 'both';
  compact?: boolean;
  /**
   * @deprecated "as" has been removed and no longer has any effect!
   */
  as?: 'document' | 'fragment';
  projectRoot?: string;
  preprocessStyle?: (content: string, attrs: Record<string, string>) => null | Promise<PreprocessorResult | PreprocessorError>;
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
      map: string;
    }
);

export interface HydratedComponent {
  exportName: string;
  specifier: string;
  resolvedPath: string;
}

export interface TransformResult {
  code: string;
  map: string;
  scope: string;
  styleError: string[];
  errors: Message[];
  warnings: Message[];
  css: string[];
  scripts: HoistedScript[];
  hydratedComponents: HydratedComponent[];
  clientOnlyComponents: HydratedComponent[];
}

export interface SourceMap {
  file: string;
  mappings: string;
  names: string[];
  sources: string[];
  sourcesContent: string[];
  version: number;
}

export interface TSXResult {
  code: string;
  map: SourceMap;
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
