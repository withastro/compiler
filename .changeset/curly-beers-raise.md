---
'@astrojs/compiler': minor
---

The compiler will now return `diagnostics` and unique error codes to be handled by the consumer. For example:

```js
import type { DiagnosticSeverity, DiagnosticCode } from '@astrojs/compiler/types';
import { transform } from '@astrojs/compiler';

async function run() {
  const { diagnostics } = await transform(file, opts);

  function log(severity: DiagnosticSeverity, message: string) {
    switch (severity) {
      case DiagnosticSeverity.Error:
        return console.error(message);
      case DiagnosticSeverity.Warning:
        return console.warn(message);
      case DiagnosticSeverity.Information:
        return console.info(message);
      case DiagnosticSeverity.Hint:
        return console.info(message);
    }
  }

  for (const diagnostic of diagnostics) {
    let message = diagnostic.text;
    if (diagnostic.hint) {
      message += `\n\n[hint] ${diagnostic.hint}`;
    }

    // Or customize messages for a known DiagnosticCode
    if (diagnostic.code === DiagnosticCode.ERROR_UNMATCHED_IMPORT) {
      message = `My custom message about an unmatched import!`;
    }
    log(diagnostic.severity, message);
  }
}
```
