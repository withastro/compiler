import sass from 'sass';

export async function preprocessStyle(value, attrs): Promise<any> {
  if (!attrs.lang) {
    return null;
  }
  if (attrs.lang === 'scss') {
    return transformSass(value);
  }
  return null;
}

export function transformSass(value: string) {
  return new Promise((resolve, reject) => {
    sass.render({ data: value }, (err, result) => {
      if (err) {
        reject(err);
        return;
      }
      resolve({ code: result.css.toString('utf8'), map: result.map });
      return;
    });
  });
}

