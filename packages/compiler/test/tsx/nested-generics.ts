import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('handles plain aliases', async () => {
  const input = `---
interface LocalImageProps {}
type Props = LocalImageProps;
---`;
  const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
  assert.ok(output.code.includes('(_props: Props)'), 'Includes aliased Props as correct props');
});

test('handles aliases with nested generics', async () => {
  const input = `---
interface LocalImageProps {
  src: Promise<{ default: string }>;
}

type Props = LocalImageProps;
---`;
  const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
  assert.ok(output.code.includes('(_props: Props)'), 'Includes aliased Props as correct props');
});

test('gracefully handles Image props', async () => {
  const input = `---
interface LocalImageProps
	extends Omit<HTMLAttributes, 'src' | 'width' | 'height'>,
		Omit<TransformOptions, 'src'>,
		Pick<astroHTML.JSX.ImgHTMLAttributes, 'loading' | 'decoding'> {
	src: ImageMetadata | Promise<{ default: ImageMetadata }>;
	/** Defines an alternative text description of the image. Set to an empty string (alt="") if the image is not a key part of the content (it's decoration or a tracking pixel). */
	alt: string;
	sizes: HTMLImageElement['sizes'];
	widths: number[];
	formats?: OutputFormat[];
}
interface RemoteImageProps
	extends Omit<HTMLAttributes, 'src' | 'width' | 'height'>,
		TransformOptions,
		Pick<ImgHTMLAttributes, 'loading' | 'decoding'> {
	src: string;
	/** Defines an alternative text description of the image. Set to an empty string (alt="") if the image is not a key part of the content (it's decoration or a tracking pixel). */
	alt: string;
	sizes: HTMLImageElement['sizes'];
	widths: number[];
	aspectRatio: TransformOptions['aspectRatio'];
	formats?: OutputFormat[];
	background: TransformOptions['background'];
}
export type Props = LocalImageProps | RemoteImageProps;
---`;
  const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
  assert.ok(output.code.includes('(_props: Props)'), 'Includes aliased Props as correct props');
});

test.run();
