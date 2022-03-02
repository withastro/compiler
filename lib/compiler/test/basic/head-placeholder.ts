import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `<html><head><title>Ah</title></head></html>`;

let result;
test.before(async () => {
  result = await transform(FIXTURE);
});

test.skip('head placeholder', () => {
  assert.not.match(result.code, '<!--astro:head-->', 'Expected output to contain <!--astro:head--> placeholder');
});

test.run();
