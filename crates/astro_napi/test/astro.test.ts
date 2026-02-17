import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

import { compileAstroSync, compileAstro } from '../index';

describe('compileAstroSync', () => {
	it('compiles a simple Astro component', () => {
		const result = compileAstroSync('<h1>Hello</h1>');
		assert.deepStrictEqual(result.diagnostics, []);
		const code = result.code;
		assert.ok(code.includes('$$render'));
		assert.ok(code.includes('<h1>Hello</h1>'));
		assert.ok(code.includes('$$createComponent'));
	});

	it('compiles frontmatter', () => {
		const result = compileAstroSync(`---
const name = "World";
---
<h1>Hello {name}!</h1>`);
		assert.deepStrictEqual(result.diagnostics, []);
		const code = result.code;
		assert.ok(code.includes('const name = "World"'));
		assert.ok(code.includes('${name}'));
	});

	it('accepts filename option', () => {
		const result = compileAstroSync('<h1>Hello</h1>', {
			filename: 'Test.astro',
		});
		assert.deepStrictEqual(result.diagnostics, []);
		assert.ok(result.code.includes('$$createComponent'));
	});

	it('always includes metadata', () => {
		const result = compileAstroSync('<h1>Hello</h1>');
		assert.deepStrictEqual(result.diagnostics, []);
		const code = result.code;
		assert.ok(code.includes('$$metadata'));
	});

	it('handles components', () => {
		const result = compileAstroSync('<Component />');
		assert.deepStrictEqual(result.diagnostics, []);
		assert.ok(result.code.includes('$$renderComponent'));
	});

	it('returns errors for invalid syntax', () => {
		const result = compileAstroSync('{ invalid js {{{');
		assert.ok(result.diagnostics.length > 0);
		assert.strictEqual(result.code, '');
	});

	it('handles MathML content', () => {
		const result = compileAstroSync('<math><annotation>R^{2x}</annotation></math>');
		assert.deepStrictEqual(result.diagnostics, []);
		assert.ok(result.code.includes('R^{2x}'));
	});

	it('returns TransformResult fields', () => {
		const result = compileAstroSync('<h1>Hello</h1>');
		assert.deepStrictEqual(result.diagnostics, []);
		assert.strictEqual(typeof result.map, 'string');
		assert.strictEqual(typeof result.scope, 'string');
		assert.deepStrictEqual(result.css, []);
		assert.deepStrictEqual(result.scripts, []);
		assert.deepStrictEqual(result.hydratedComponents, []);
		assert.deepStrictEqual(result.clientOnlyComponents, []);
		assert.deepStrictEqual(result.serverComponents, []);
		assert.strictEqual(typeof result.containsHead, 'boolean');
		assert.strictEqual(typeof result.propagation, 'boolean');
		assert.deepStrictEqual(result.styleError, []);
		assert.deepStrictEqual(result.diagnostics, []);
	});

	it('detects explicit <head> element', () => {
		const result = compileAstroSync(
			'<html><head><title>Test</title></head><body><h1>Hi</h1></body></html>',
		);
		assert.deepStrictEqual(result.diagnostics, []);
		assert.strictEqual(result.containsHead, true);
	});

	it('reports containsHead false when no head', () => {
		const result = compileAstroSync('<h1>Hello</h1>');
		assert.deepStrictEqual(result.diagnostics, []);
		assert.strictEqual(result.containsHead, false);
	});
});

describe('compileAstro (async)', () => {
	it('compiles a simple Astro component', async () => {
		const result = await compileAstro('<h1>Hello</h1>');
		assert.deepStrictEqual(result.diagnostics, []);
		assert.ok(result.code.includes('$$render'));
	});

	it('returns TransformResult fields', async () => {
		const result = await compileAstro('<h1>Hello</h1>');
		assert.deepStrictEqual(result.diagnostics, []);
		assert.strictEqual(typeof result.map, 'string');
		assert.strictEqual(typeof result.scope, 'string');
		assert.deepStrictEqual(result.css, []);
		assert.deepStrictEqual(result.scripts, []);
	});
});
