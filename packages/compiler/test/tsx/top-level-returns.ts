import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { TSXPrefix } from '../utils.js';

test('transforms top-level returns to throw statements', async () => {
	const input = `---
if (something) {
	return Astro.redirect();
}

function thatDoesSomething() {
	return "Hey";
}

class Component {
	render() {
		return "wow"!
	}
}
---`;
	const output = `${TSXPrefix}
if (something) {
	throw  Astro.redirect();
}

function thatDoesSomething() {
	return "Hey";
}

class Component {
	render() {
		return "wow"!
	}
}


export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test.run();
