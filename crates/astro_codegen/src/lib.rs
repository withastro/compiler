//! Astro Codegen
//!
//! Transforms Astro AST (`AstroRoot`) into JavaScript code that can be executed
//! by the Astro runtime.
//!
//! ## Output Format
//!
//! The generated JavaScript follows the Astro compiler output format:
//!
//! ```js
//! import { Fragment, render as $$render, ... } from "astro/runtime/server/index.js";
//! // User imports from frontmatter
//!
//! const $$Component = $$createComponent(($$result, $$props, $$slots) => {
//!     // Non-import frontmatter code
//!     return $$render`...template...`;
//! }, 'filename', undefined);
//! export default $$Component;
//! ```

mod options;
mod printer;
pub mod scanner;

pub use options::{ScopedStyleStrategy, TransformOptions};
pub use printer::{
    AstroCodegen, HoistedScriptType, TransformResult, TransformResultHoistedScript,
    TransformResultHydratedComponent, transform,
};
pub use scanner::{AstroScanner, ScanResult};
