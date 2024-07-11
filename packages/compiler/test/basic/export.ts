import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('TypeScript props on newline', async () => {
	const FIXTURE = `---
export type Props = BaseLayoutProps &
  Pick<LocaleSelectProps, 'locale' | 'someOtherProp'>;
---

<div></div>
`;
	const result = await transform(FIXTURE);
	assert.match(
		result.code,
		'BaseLayoutProps &\n  Pick<',
		'Expected output to contain full Props export'
	);
});

test('exported type', async () => {
	const FIXTURE = `---
// this is fine
export type NumberType = number;
// astro hangs because of this typedef.
// comment it out and astro will work fine.
export type FuncType = (x: number) => number;
---

{new Date()}
`;
	const result = await transform(FIXTURE);
	assert.match(
		result.code,
		'export type NumberType = number;\nexport type FuncType = (x: number) => number',
		'Expected output to contain full Props export'
	);
});

test.run();
