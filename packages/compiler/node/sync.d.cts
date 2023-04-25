import type * as types from '../shared/types';
type UnwrappedPromise<T> = T extends (...params: any) => Promise<infer Return> ? (...params: Parameters<T>) => Return : T;
interface Service {
    transform: UnwrappedPromise<typeof types.transform>;
    parse: UnwrappedPromise<typeof types.parse>;
    convertToTSX: UnwrappedPromise<typeof types.convertToTSX>;
}
export declare const transform: (input: string, options: types.TransformOptions | undefined) => types.TransformResult;
export declare const parse: (input: string, options: types.ParseOptions | undefined) => types.ParseResult;
export declare const convertToTSX: (input: string, options: types.ConvertToTSXOptions | undefined) => types.TSXResult;
export declare function startRunningService(): Service;
export {};
