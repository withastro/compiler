import { defineConfig } from 'tsup';

export default defineConfig((options) => ({
	entry: ['src/**'],
	outDir: 'dist',
	format: ['cjs', 'esm'],
	dts: true,
	clean: true,
	minify: !options.watch,
	sourcemap: Boolean(options.watch),
	watch: options.watch,
	shims: true,
	external: ['@astrojs/compiler-binding'],
}));
