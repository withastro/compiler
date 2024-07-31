import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { TSXPrefix } from '../utils.js';

test('transforms top-level returns to throw statements', async () => {
	const input = `---
import type { GetStaticPaths } from "astro";

function thatDoesSomething() {
	return "Hey";
}

class Component {
	render() {
		return "wow"!
	}
}

export const getStaticPaths = (({ paginate }) => {
	const data = [0, 1, 2];
	return paginate(data, {
		pageSize: 10,
	});
}) satisfies GetStaticPaths;

export async function getStaticPaths({
	paginate,
}: {
paginate: PaginateFunction;
}) {
	const { data: products }: { data: IProduct[] } = await getEntry(
		"products",
		"products,
	);

	return paginate(products, {
		pageSize: 10,
	});
}

const something = {
	someFunction: () => {
		return "Hello World";
	},
	someOtherFunction() {
		return "Hello World";
	},
};

if (something) {
	return Astro.redirect();
}
---`;
	const output = `${TSXPrefix}
import type { GetStaticPaths } from "astro";

function thatDoesSomething() {
	return "Hey";
}

class Component {
	render() {
		return "wow"!
	}
}

export const getStaticPaths = (({ paginate }) => {
	const data = [0, 1, 2];
	return paginate(data, {
		pageSize: 10,
	});
}) satisfies GetStaticPaths;

export async function getStaticPaths({
	paginate,
}: {
paginate: PaginateFunction;
}) {
	const { data: products }: { data: IProduct[] } = await getEntry(
		"products",
		"products,
	);

	return paginate(products, {
		pageSize: 10,
	});
}

const something = {
	someFunction: () => {
		return "Hello World";
	},
	someOtherFunction() {
		return "Hello World";
	},
};

if (something) {
	throw  Astro.redirect();
}


export default function __AstroComponent_(_props: ASTRO__MergeUnion<ASTRO__Get<ASTRO__InferredGetStaticPath, 'props'>>): any {}
type ASTRO__ArrayElement<ArrayType extends readonly unknown[]> = ArrayType extends readonly (infer ElementType)[] ? ElementType : never;
type ASTRO__Flattened<T> = T extends Array<infer U> ? ASTRO__Flattened<U> : T;
type ASTRO__InferredGetStaticPath = ASTRO__Flattened<ASTRO__ArrayElement<Awaited<ReturnType<typeof getStaticPaths>>>>;
type ASTRO__MergeUnion<T, K extends PropertyKey = T extends unknown ? keyof T : never> = T extends unknown ? T & { [P in Exclude<K, keyof T>]?: never } extends infer O ? { [P in keyof O]: O[P] } : never : never;
type ASTRO__Get<T, K> = T extends undefined ? undefined : K extends keyof T ? T[K] : never;
/**
 * Astro global available in all contexts in .astro files
 *
 * [Astro documentation](https://docs.astro.build/reference/api-reference/#astro-global)
*/
declare const Astro: Readonly<import('astro').AstroGlobal<ASTRO__MergeUnion<ASTRO__Get<ASTRO__InferredGetStaticPath, 'props'>>, typeof __AstroComponent_, ASTRO__Get<ASTRO__InferredGetStaticPath, 'params'>>>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test.run();
