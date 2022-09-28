import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

test('TypeScript props on newline', async () => {
  const FIXTURE = `---
export type Props = BaseLayoutProps &
  Pick<LocaleSelectProps, 'locale' | 'someOtherProp'>;
---

<div></div>
`;
  const result = await transform(FIXTURE);
  assert.match(result.code, 'BaseLayoutProps &\n  Pick<', 'Expected output to contain full Props export');
});

test.run();
