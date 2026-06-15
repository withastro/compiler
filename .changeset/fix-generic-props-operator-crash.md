---
"@astrojs/compiler": patch
---

Fix a crash when converting a component to TSX whose `Props` generic contains an operator that the lexer emits as a single `<`/`>` token, such as `interface Props<T extends number = (1 << 2)>`. These operators were mistaken for generic angle brackets, desyncing the bracket-depth tracking and panicking with a slice bounds error.
