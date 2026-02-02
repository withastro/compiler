---
"@astrojs/compiler": patch
---

Fixed an issue where explicit `<html>` and `<head>` tags were removed from output when a JSX comment appeared between DOCTYPE and the `<html>` tag.
