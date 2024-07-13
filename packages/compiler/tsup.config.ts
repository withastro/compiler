import { defineConfig } from 'tsup';

export default defineConfig((options) => ({
	entry: ['src/node/**', 'src/browser/**', 'src/shared/**'],
	outDir: 'dist',
	format: ['cjs', 'esm'],
	dts: true,
	clean: true,
	minify: !options.watch,
	sourcemap: Boolean(options.watch),
	watch: options.watch,
	publicDir: 'wasm',
	shims: true,
}));
