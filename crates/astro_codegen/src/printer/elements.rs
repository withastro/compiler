//! HTML element printing, attributes, and element utilities.
//!
//! Contains `impl AstroCodegen` methods for rendering plain HTML elements
//! (non-component), including attribute handling, `set:html`/`set:text`
//! directives on HTML elements, `<slot>` element rendering, transition
//! attributes, and element classification helpers (`is_void_element`,
//! `is_head_element`).

use oxc_ast::ast::*;
use oxc_codegen::{Codegen, Context, GenExpr};

use super::AstroCodegen;
use super::escape::{escape_double_quotes, escape_html_attribute};
use super::runtime;
use crate::scanner::get_jsx_attribute_name;

/// Returns `true` for HTML void elements that must not have a closing tag.
pub(super) fn is_void_element(name: &str) -> bool {
    matches!(
        name,
        "area"
            | "base"
            | "br"
            | "col"
            | "embed"
            | "hr"
            | "img"
            | "input"
            | "link"
            | "meta"
            | "param"
            | "selectedcontent" // HTML customizable select element
            | "source"
            | "track"
            | "wbr"
    )
}

/// Elements that can appear in `<head>` and should NOT trigger `$$maybeRenderHead`.
pub(super) fn is_head_element(name: &str) -> bool {
    matches!(
        name,
        "html"
            | "head"
            | "base"
            | "basefont"
            | "bgsound"
            | "link"
            | "meta"
            | "noframes"
            | "script"
            | "style"
            | "template"
            | "title"
    )
}

impl<'a> AstroCodegen<'a> {
    /// Print an HTML (non-component) element.
    pub(super) fn print_html_element(&mut self, el: &JSXElement<'a>, name: &str) {
        // Handle <slot> element specially — it's a slot placeholder, not an HTML element.
        // Unless it has `is:inline`, in which case render as raw HTML.
        if name == "slot" && !Self::has_is_inline_attribute(&el.opening_element.attributes) {
            self.print_slot_element(el);
            return;
        }

        // Check if this is a special element
        let is_head = name == "head";
        let was_in_head = self.in_head;

        if is_head {
            self.in_head = true;
            self.has_explicit_head = true;
        }

        // Insert $$maybeRenderHead before the first body HTML element
        self.maybe_insert_render_head(name);

        // Extract set:html and set:text directives
        let set_directive = Self::extract_set_directive(&el.opening_element.attributes);

        // Opening tag
        self.print("<");
        self.print(name);

        // Attributes (excluding set:html and set:text)
        self.print_html_attributes(&el.opening_element.attributes);

        // Close the opening tag and handle content
        self.print(">");

        // Handle special head insertion
        if is_head {
            // Children
            for child in &el.children {
                self.print_jsx_child(child);
            }
            // Insert renderHead before closing head tag
            self.print(&format!(
                "${{{}({})}}",
                runtime::RENDER_HEAD,
                runtime::RESULT
            ));
            // Mark that head rendering is done — prevents $$maybeRenderHead from being inserted later.
            self.render_head_inserted = true;
        } else if let Some((directive_type, value, needs_unescape, is_raw_text)) = set_directive {
            // set:html or set:text directive — inject the content
            if is_raw_text {
                // set:text with string literal — inline raw text without ${}
                self.print(&value);
            } else if directive_type == "html" && needs_unescape {
                // Only use $$unescapeHTML for non-literal expressions
                self.print(&format!("${{{}({})}}", runtime::UNESCAPE_HTML, value));
            } else {
                // For literals (string/template) or set:text expression, just interpolate
                self.print(&format!("${{{value}}}"));
            }
        } else {
            // Regular children
            for child in &el.children {
                self.print_jsx_child(child);
            }
        }

        // Closing tag (skip for void elements like <meta>, <input>, <br>, etc.)
        if !is_void_element(name) {
            self.print("</");
            self.print(name);
            self.print(">");
        }

        if is_head {
            self.in_head = was_in_head;
        }
    }

    /// Print a `<slot>` element as a `$$renderSlot` call.
    ///
    /// - `<slot />` → `$$renderSlot($$result, $$slots["default"])`
    /// - `<slot name="foo" />` → `$$renderSlot($$result, $$slots["foo"])`
    /// - `<slot><p>fallback</p></slot>` → `$$renderSlot($$result, $$slots["default"], $$render\`<p>fallback</p>\`)`
    fn print_slot_element(&mut self, el: &JSXElement<'a>) {
        // Extract slot name from attributes (default is "default")
        let slot_name = Self::extract_slot_name(&el.opening_element.attributes);

        self.print("${");
        self.print(runtime::RENDER_SLOT);
        self.print("(");
        self.print(runtime::RESULT);
        self.print(",$$slots[\"");
        self.print(&slot_name);
        self.print("\"]");

        // Add fallback content if there are children
        if !el.children.is_empty() {
            self.print(",");
            self.print(runtime::RENDER);
            self.print("`");
            for child in &el.children {
                self.print_jsx_child(child);
            }
            self.print("`");
        }

        self.print(")}");
    }

    /// Extract the `name` attribute from a slot element, defaulting to `"default"`.
    fn extract_slot_name(attrs: &[JSXAttributeItem<'a>]) -> String {
        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let attr_name = get_jsx_attribute_name(&attr.name);
                if attr_name == "name" {
                    if let Some(JSXAttributeValue::StringLiteral(lit)) = &attr.value {
                        return lit.value.to_string();
                    }
                    if let Some(JSXAttributeValue::ExpressionContainer(expr)) = &attr.value {
                        // Dynamic slot name
                        let mut codegen = Codegen::new();
                        if let Some(e) = expr.expression.as_expression() {
                            e.print_expr(
                                &mut codegen,
                                oxc_syntax::precedence::Precedence::Lowest,
                                Context::default().with_typescript(),
                            );
                        }
                        return format!("\" + {} + \"", codegen.into_source_text());
                    }
                }
            }
        }
        "default".to_string()
    }

    /// Check if an element has the `is:inline` attribute.
    pub(super) fn has_is_inline_attribute(attrs: &[JSXAttributeItem<'a>]) -> bool {
        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                if name == "is:inline" {
                    return true;
                }
            }
        }
        false
    }

    /// Extract `set:html`/`set:text` directive from HTML element attributes.
    ///
    /// Returns `(directive_type, value, needs_unescape, is_raw_text)`:
    /// - `is_raw_text` is `true` for `set:text` with string literal (should be inlined without `${}`)
    pub(super) fn extract_set_directive(
        attrs: &[JSXAttributeItem<'a>],
    ) -> Option<(&'static str, String, bool, bool)> {
        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                if name == "set:html" || name == "set:text" {
                    let directive_type = if name == "set:html" { "html" } else { "text" };
                    let (value, needs_unescape, is_raw_text) = match &attr.value {
                        Some(JSXAttributeValue::StringLiteral(lit)) => {
                            if directive_type == "text" {
                                // set:text with string literal: inline raw text without ${}
                                (lit.value.as_str().to_string(), false, true)
                            } else {
                                // set:html with string literal: use ${"content"}
                                (
                                    format!("\"{}\"", escape_double_quotes(lit.value.as_str())),
                                    false,
                                    false,
                                )
                            }
                        }
                        Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                            let mut codegen = Codegen::new();
                            let mut is_literal = false;
                            if let Some(e) = expr.expression.as_expression() {
                                is_literal = matches!(
                                    e,
                                    Expression::StringLiteral(_) | Expression::TemplateLiteral(_)
                                );
                                e.print_expr(
                                    &mut codegen,
                                    oxc_syntax::precedence::Precedence::Lowest,
                                    Context::default().with_typescript(),
                                );
                            }
                            let needs_unescape = directive_type == "html" && !is_literal;
                            (codegen.into_source_text(), needs_unescape, false)
                        }
                        _ => ("void 0".to_string(), false, false),
                    };
                    return Some((directive_type, value, needs_unescape, is_raw_text));
                }
            }
        }
        None
    }

    /// Print all HTML attributes for an element, handling transition directives,
    /// `class`/`class:list` merging, and spread attributes.
    pub(super) fn print_html_attributes(&mut self, attrs: &[JSXAttributeItem<'a>]) {
        // Check for class + class:list combination that needs merging
        let mut static_class: Option<&str> = None;
        let mut class_list_expr: Option<&JSXExpressionContainer<'a>> = None;

        // Check for transition attributes
        let mut transition_name: Option<String> = None;
        let mut transition_animate: Option<String> = None;
        let mut transition_persist: Option<String> = None;

        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                if name == "class" {
                    if let Some(JSXAttributeValue::StringLiteral(lit)) = &attr.value {
                        static_class = Some(lit.value.as_str());
                    }
                } else if name == "class:list" {
                    if let Some(JSXAttributeValue::ExpressionContainer(expr)) = &attr.value {
                        class_list_expr = Some(expr);
                    }
                } else if name == "transition:name" {
                    transition_name = Some(Self::get_attr_value_string(attr));
                } else if name == "transition:animate" {
                    transition_animate = Some(Self::get_attr_value_string(attr));
                } else if name == "transition:persist" || name == "transition:persist-props" {
                    transition_persist = Some(Self::get_attr_value_string_or_empty(attr));
                }
            }
        }

        // If both class and class:list exist, merge them
        let has_merged_class = static_class.is_some() && class_list_expr.is_some();

        // Handle transition:persist — if there's a transition:name, use that for persist value.
        // Otherwise use $$createTransitionScope.
        if let Some(_persist_val) = &transition_persist {
            if let Some(name_val) = &transition_name {
                let clean_val = name_val.trim_matches('"');
                self.print(&format!(" data-astro-transition-persist=\"{clean_val}\""));
            } else {
                let hash = self.generate_transition_hash();
                self.print(&format!(
                    "${{{}({}({}, \"{}\"), \"data-astro-transition-persist\")}}",
                    runtime::ADD_ATTRIBUTE,
                    runtime::CREATE_TRANSITION_SCOPE,
                    runtime::RESULT,
                    hash
                ));
            }
        }

        // Handle transition:name and transition:animate together
        if transition_name.is_some() || transition_animate.is_some() {
            let name_val = transition_name.unwrap_or_else(|| "\"\"".to_string());
            let animate_val = transition_animate.unwrap_or_else(|| "\"\"".to_string());
            let hash = self.generate_transition_hash();
            self.print(&format!(
                "${{{}({}({}, \"{}\", {}, {}), \"data-astro-transition-scope\")}}",
                runtime::ADD_ATTRIBUTE,
                runtime::RENDER_TRANSITION,
                runtime::RESULT,
                hash,
                animate_val,
                name_val
            ));
        }

        for attr in attrs {
            match attr {
                JSXAttributeItem::Attribute(attr) => {
                    let name = get_jsx_attribute_name(&attr.name);
                    // Skip set:html and set:text — handled separately
                    if name == "set:html" || name == "set:text" {
                        continue;
                    }
                    // Skip slot attribute if we're inside a conditional slot context
                    if self.skip_slot_attribute && name == "slot" {
                        continue;
                    }
                    // Skip is:inline and is:raw — Astro directives, not HTML attributes
                    if name == "is:inline" || name == "is:raw" {
                        continue;
                    }
                    // Skip transition directives — already handled above
                    if name.starts_with("transition:") {
                        continue;
                    }
                    // Skip individual class if we're merging with class:list
                    if has_merged_class && name == "class" {
                        continue;
                    }
                    // Handle merged class:list
                    if has_merged_class && name == "class:list" {
                        if let (Some(static_val), Some(expr)) = (static_class, class_list_expr) {
                            self.print(&format!("${{{}([", runtime::ADD_ATTRIBUTE));
                            self.print(&format!("\"{}\"", escape_double_quotes(static_val)));
                            self.print(", ");
                            self.print_jsx_expression(&expr.expression);
                            self.print("], \"class:list\")}");
                        }
                        continue;
                    }
                    self.print_html_attribute(attr);
                }
                JSXAttributeItem::SpreadAttribute(spread) => {
                    self.print(&format!("${{{}(", runtime::SPREAD_ATTRIBUTES));
                    self.print_expression(&spread.argument);
                    self.print(")}");
                }
            }
        }
    }

    /// Get attribute value as a string representation for codegen.
    pub(super) fn get_attr_value_string(attr: &JSXAttribute<'a>) -> String {
        match &attr.value {
            Some(JSXAttributeValue::StringLiteral(lit)) => {
                format!("\"{}\"", escape_double_quotes(lit.value.as_str()))
            }
            Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                if let Some(e) = expr.expression.as_expression() {
                    let mut codegen = Codegen::new();
                    e.print_expr(
                        &mut codegen,
                        oxc_syntax::precedence::Precedence::Lowest,
                        Context::default().with_typescript(),
                    );
                    let source = codegen.into_source_text();
                    // Template literals don't need parens, but other expressions do
                    if matches!(e, Expression::TemplateLiteral(_)) {
                        source
                    } else {
                        format!("({source})")
                    }
                } else {
                    "\"\"".to_string()
                }
            }
            _ => "\"\"".to_string(),
        }
    }

    /// Get attribute value as a string, or empty string if no value (for boolean attrs).
    pub(super) fn get_attr_value_string_or_empty(attr: &JSXAttribute<'a>) -> String {
        match &attr.value {
            None => String::new(),
            _ => Self::get_attr_value_string(attr),
        }
    }

    /// Generate a hash for transition scope.
    pub(super) fn generate_transition_hash(&mut self) -> String {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};

        // Increment counter to get unique index
        let counter = self.transition_counter;
        self.transition_counter += 1;

        // Hash the combination of source hash + counter (like Go's "%s-%v" format)
        let mut hasher = DefaultHasher::new();
        format!("{}-{}", self.source_hash, counter).hash(&mut hasher);
        let hash = hasher.finish();

        // Convert to base32-like lowercase string (8 chars)
        Self::to_base32_like(hash)
    }

    /// Print a single HTML attribute (static or dynamic).
    fn print_html_attribute(&mut self, attr: &JSXAttribute<'a>) {
        let name = get_jsx_attribute_name(&attr.name);

        match &attr.value {
            None => {
                // Boolean attribute
                self.print(" ");
                self.print(&name);
            }
            Some(value) => match value {
                JSXAttributeValue::StringLiteral(lit) => {
                    self.print(" ");
                    self.print(&name);
                    self.print("=\"");
                    self.print(&escape_html_attribute(lit.value.as_str()));
                    self.print("\"");
                }
                JSXAttributeValue::ExpressionContainer(expr) => {
                    // Dynamic attribute
                    self.print(&format!("${{{}(", runtime::ADD_ATTRIBUTE));
                    self.print_jsx_expression(&expr.expression);
                    self.print(", \"");
                    self.print(&name);
                    self.print("\")}");
                }
                JSXAttributeValue::Element(el) => {
                    // JSX element as attribute value (rare)
                    self.print(" ");
                    self.print(&name);
                    self.print("=\"");
                    self.print_jsx_element(el);
                    self.print("\"");
                }
                JSXAttributeValue::Fragment(frag) => {
                    self.print(" ");
                    self.print(&name);
                    self.print("=\"");
                    self.print_jsx_fragment(frag);
                    self.print("\"");
                }
            },
        }
    }
}
