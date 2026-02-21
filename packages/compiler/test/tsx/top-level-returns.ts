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

test('preserves returns inside arrow functions', async () => {
	const input = `---
const foo = () => {
	return "value";
}

if (condition) {
	return Astro.redirect("/login");
}
---`;
	const output = `${TSXPrefix}
const foo = () => {
	return "value";
}

if (condition) {
	throw  Astro.redirect("/login");
}


export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('preserves returns inside object methods', async () => {
	const input = `---
const something = {
	someFunction: () => {
		return "Hello World";
	},
	someOtherFunction() {
		return "Hello World";
	},
};

if (true) {
	return Astro.redirect();
}
---`;
	const output = `${TSXPrefix}
const something = {
	someFunction: () => {
		return "Hello World";
	},
	someOtherFunction() {
		return "Hello World";
	},
};

if (true) {
	throw  Astro.redirect();
}


export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('handles multiple top-level returns', async () => {
	const input = `---
if (condition1) {
	return Astro.redirect("/a");
}

if (condition2) {
	return Astro.redirect("/b");
}
---`;
	const output = `${TSXPrefix}
if (condition1) {
	throw  Astro.redirect("/a");
}

if (condition2) {
	throw  Astro.redirect("/b");
}


export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('no transformation when no top-level returns', async () => {
	const input = `---
function foo() {
	return "bar";
}

const arrow = () => {
	return "baz";
}
---`;
	const output = `${TSXPrefix}
function foo() {
	return "bar";
}

const arrow = () => {
	return "baz";
}


export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('handles TypeScript syntax without losing returns', async () => {
	const input = `---
type Response = { status: number };
const handler = (input: string): Response => {
	return { status: 200 };
};

const value = (foo as string);

if (value) {
	return Astro.redirect('/ok');
}
---`;
	const output = `${TSXPrefix}
type Response = { status: number };
const handler = (input: string): Response => {
	return { status: 200 };
};

const value = (foo as string);

if (value) {
	throw  Astro.redirect('/ok');
}


export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('does not transform returns in nested class or object methods', async () => {
	const input = `---
class Foo {
	method() {
		return 'foo';
	}
	arrow = () => {
		return 'bar';
	}
}

const obj = {
	method() {
		return 'baz';
	},
};

if (true) {
	return Astro.redirect('/nested');
}
---`;
	const output = `${TSXPrefix}
class Foo {
	method() {
		return 'foo';
	}
	arrow = () => {
		return 'bar';
	}
}

const obj = {
	method() {
		return 'baz';
	},
};

if (true) {
	throw  Astro.redirect('/nested');
}


export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('handles computed methods and generic arrows', async () => {
	const input = `---
class Foo {
	['get']() {
		return 'ok';
	}

	static ['load']() {
		return 'static';
	}
}

const obj = {
	['get']() {
		return 'obj';
	},
};

const generic = <T,>(value: T) => {
	return value;
};

if (true) {
	return Astro.redirect('/computed');
}
---`;
	const output = `${TSXPrefix}
class Foo {
	['get']() {
		return 'ok';
	}

	static ['load']() {
		return 'static';
	}
}

const obj = {
	['get']() {
		return 'obj';
	},
};

const generic = <T,>(value: T) => {
	return value;
};

if (true) {
	throw  Astro.redirect('/computed');
}


export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('handles satisfies and as const in top-level conditionals', async () => {
	const input = `---
const config = {
	flag: true,
} as const;

const map = {
	name: 'astro',
} satisfies Record<string, string>;

if (config.flag) {
	return Astro.redirect('/satisfies');
}
---`;
	const output = `${TSXPrefix}
const config = {
	flag: true,
} as const;

const map = {
	name: 'astro',
} satisfies Record<string, string>;

if (config.flag) {
	throw  Astro.redirect('/satisfies');
}


export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('handles type-position arrows without missing top-level returns', async () => {
	const input = `---
type Fn = () => void;
type Factory = (value: string) => { ok: boolean };

if (true) {
	return Astro.redirect('/types');
}
---`;
	const output = `${TSXPrefix}
type Fn = () => void;
type Factory = (value: string) => { ok: boolean };

if (true) {
	throw  Astro.redirect('/types');
}


export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test.run();
