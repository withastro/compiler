const ESCAPE = /[&"<]/g;
const CHARS: Record<string, string> = {
  '"': '&quot;',
  '&': '&amp;',
  '<': '&lt'
};

export function escape(value: string) {
  if (typeof value !== 'string') {
    return value;
  }
  let last = (ESCAPE.lastIndex = 0),
    tmp = 0,
    out = '';
  while (ESCAPE.test(value.valueOf())) {
    tmp = ESCAPE.lastIndex - 1;
    out += value.substring(last, tmp) + CHARS[value[tmp]];
    last = tmp + 1;
  }
  return out + value.substring(last);
}
