---
'@astrojs/compiler': minor
---

Switch from TinyGo to Go's built-in WASM output. While this is an unfortunate size increase for our `.wasm` file, it should also be significantly more stable and cut down on hard-to-reproduce bugs.

Please see https://github.com/withastro/compiler/pull/291 for more details.
