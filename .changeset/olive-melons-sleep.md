---
"@astrojs/compiler": minor
---

Adds two new options to `convertToTSX`: `includeScripts` and `includeStyles`. These options allow you to optionally remove scripts and styles from the output TSX file.

Additionally this PR makes it so scripts and styles metadata are now included in the `metaRanges` property of the result of `convertToTSX`. This is notably useful in order to extract scripts and styles from the output TSX file into separate files for language servers.
