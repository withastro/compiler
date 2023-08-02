import type { RootNode } from './ast';
import type { DiagnosticCode } from './diagnostics';
export type * from './ast';

export interface PreprocessorResult {
  code: string;
  map?: string;
}

export interface PreprocessorError {
  error: string;
}

export interface ParseOptions {
  position?: boolean;
}

// eslint-disable-next-line no-shadow
export enum DiagnosticSeverity {
  Error = 1,
  Warning = 2,
  Information = 3,
  Hint = 4,
}

export interface DiagnosticMessage {
  severity: DiagnosticSeverity;
  code: DiagnosticCode;
  location: DiagnosticLocation;
  hint?: string;
  text: string;
}

export interface DiagnosticLocation {
  file: string;
  // 1-based
  line: number;
  // 1-based
  column: number;
  length: number;
}

export interface TransformOptions {
  internalURL?: string;
  filename?: string;
  normalizedFilename?: string;
  sourcemap?: boolean | 'inline' | 'external' | 'both';
  astroGlobalArgs?: string;
  compact?: boolean;
  resultScopedSlot?: boolean;
  scopedStyleStrategy?: 'where' | 'class' | 'attribute';
  /**
   * @deprecated "as" has been removed and no longer has any effect!
   */
  as?: 'document' | 'fragment';
  experimentalTransitions?: boolean;
  experimentalPersistence?: boolean;
  transitionsAnimationURL?: string;
  resolvePath?: (specifier: string) => Promise<string>;
  preprocessStyle?: (content: string, attrs: Record<string, string>) => null | Promise<PreprocessorResult | PreprocessorError>;
}

export type ConvertToTSXOptions = Pick<TransformOptions, 'filename' | 'normalizedFilename'>;

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
  diagnostics: DiagnosticMessage[];
  css: string[];
  scripts: HoistedScript[];
  hydratedComponents: HydratedComponent[];
  clientOnlyComponents: HydratedComponent[];
  containsHead: boolean;
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
  diagnostics: DiagnosticMessage[];
}

export interface ParseResult {
  ast: RootNode;
  diagnostics: DiagnosticMessage[];
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

export declare function convertToTSX(input: string, options?: ConvertToTSXOptions): Promise<TSXResult>;

// This configures the browser-based version of astro. It is necessary to
// call this first and wait for the returned promise to be resolved before
// making other API calls when using astro in the browser.
//
// Works in node: yes
// Works in browser: yes ("options" is required)
export declare function initialize(options: InitializeOptions): Promise<void>;

/**
 * When calling the core compiler APIs, e.g. `transform`, `parse`, etc, they
 * would automatically instantiate a WASM instance to process the input. When
 * done, you can call this to manually teardown the WASM instance.
 *
 * If the APIs are called again, they will automatically instantiate a new WASM
 * instance. In browsers, you have to call `initialize()` again before using the APIs.
 *
 * Note: Calling teardown is optional and exists mostly as an optimization only.
 */
export declare function teardown(): void;

export interface InitializeOptions {
  // The URL of the "astro.wasm" file. This must be provided when running
  // astro in the browser.
  wasmURL?: string;
}
