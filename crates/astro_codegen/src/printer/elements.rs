//! HTML element printing, attributes, and element utilities.
//!
//! Contains `impl AstroCodegen` methods for rendering plain HTML elements
//! (non-component), including attribute handling, `set:html`/`set:text`
//! directives on HTML elements, `<slot>` element rendering, transition
//! attributes, and element classification helpers (`is_void_element`,
//! `is_head_element`).

use super::escape::{escape_double_quotes, escape_html_attribute};
use super::runtime;
use super::{AstroCodegen, expr_to_string};
use crate::css_scoping;
use crate::options::ScopedStyleStrategy;
use crate::scanner::get_jsx_attribute_name;
use oxc_ast::ast::*;

/// Scope identifier for an element — either a CSS class or a data attribute,
/// depending on the `scopedStyleStrategy`.
#[derive(Clone)]
pub(super) enum ScopeId {
    /// `where` or `class` strategy: inject `class="astro-{hash}"`.
    Class(String),
    /// `attribute` strategy: inject `data-astro-cid-{hash}` as a boolean attribute.
    DataAttribute(String),
}

impl ScopeId {
    /// The value to embed in class lists / spread attributes (e.g. `"astro-{hash}"`).
    pub(super) fn class_value(&self) -> String {
        match self {
            ScopeId::Class(v) => v.clone(),
            ScopeId::DataAttribute(v) => format!("astro-{v}"),
        }
    }

    /// The attribute name for the `attribute` strategy (e.g. `"data-astro-cid-{hash}"`).
    pub(super) fn data_attr_name(&self) -> String {
        match self {
            ScopeId::DataAttribute(v) => format!("data-astro-cid-{v}"),
            ScopeId::Class(_) => unreachable!("data_attr_name called on Class variant"),
        }
    }

    pub(super) fn is_attribute_strategy(&self) -> bool {
        matches!(self, ScopeId::DataAttribute(_))
    }
}

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
        self.add_source_mapping_for_span(el.opening_element.span);
        // Handle <slot> element specially — it's a slot placeholder, not an HTML element.
        // Unless it has `is:inline`, in which case render as raw HTML.
        if name == "slot" && !Self::has_is_inline_attribute(&el.opening_element.attributes) {
            self.print_slot_element(el);
            return;
        }

        // Check if this is a special element
        let is_head = name == "head";
        let was_in_head = self.in_head;

        // Track non-hoistable context for nested style elements
        let is_non_hoistable = matches!(name, "svg" | "noscript" | "template");
        let was_in_non_hoistable = self.in_non_hoistable;
        if is_non_hoistable {
            self.in_non_hoistable = true;
        }

        if is_head {
            self.in_head = true;
            self.has_explicit_head = true;
        }

        // Insert $$maybeRenderHead before the first body HTML element
        self.maybe_insert_render_head(name);

        // Extract set:html and set:text directives
        let set_directive = Self::extract_set_directive(&el.opening_element.attributes);

        // Determine if this element should receive a scope identifier
        let scope_id = if self.has_scoped_styles && css_scoping::should_scope_element(name) {
            let hash = &self.source_hash;
            match self.options.scoped_style_strategy() {
                ScopedStyleStrategy::Attribute => Some(ScopeId::DataAttribute(hash.clone())),
                _ => Some(ScopeId::Class(format!("astro-{hash}"))),
            }
        } else {
            None
        };

        // Opening tag
        self.print("<");
        self.print(name);

        // Determine if this element should receive $$definedVars style injection
        let inject_define_vars = self.should_inject_define_vars(name);

        // Attributes (excluding set:html and set:text), with scope injection
        self.print_html_attributes(
            &el.opening_element.attributes,
            scope_id.as_ref(),
            inject_define_vars,
        );

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
        } else if let Some((directive_type, value, needs_unescape, is_raw_text, set_span)) =
            set_directive
        {
            // set:html or set:text directive — inject the content
            self.add_source_mapping_for_span(set_span);
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
            // Map closing tag to its source position so it appears in the sourcemap.
            if let Some(ref closing) = el.closing_element {
                self.add_source_mapping_for_span(closing.span);
            }
            self.print("</");
            self.print(name);
            self.print(">");
        }

        if is_head {
            self.in_head = was_in_head;
        }
        if is_non_hoistable {
            self.in_non_hoistable = was_in_non_hoistable;
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

        // Map the closing </slot> tag to its source position (if present).
        if let Some(ref closing) = el.closing_element {
            self.add_source_mapping_for_span(closing.span);
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
                        let expr_str = expr
                            .expression
                            .as_expression()
                            .map(expr_to_string)
                            .unwrap_or_default();
                        return format!("\" + {expr_str} + \"");
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
    /// Returns `(directive_type, value, needs_unescape, is_raw_text, span)`:
    /// - `is_raw_text` is `true` for `set:text` with string literal (should be inlined without `${}`)
    pub(super) fn extract_set_directive(
        attrs: &[JSXAttributeItem<'a>],
    ) -> Option<(&'static str, String, bool, bool, oxc_span::Span)> {
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
                            let mut value_str = String::new();
                            let mut is_literal = false;
                            if let Some(e) = expr.expression.as_expression() {
                                is_literal = matches!(
                                    e,
                                    Expression::StringLiteral(_) | Expression::TemplateLiteral(_)
                                );
                                value_str = expr_to_string(e);
                            }
                            let needs_unescape = directive_type == "html" && !is_literal;
                            (value_str, needs_unescape, false)
                        }
                        _ => ("void 0".to_string(), false, false),
                    };
                    return Some((
                        directive_type,
                        value,
                        needs_unescape,
                        is_raw_text,
                        attr.span,
                    ));
                }
            }
        }
        None
    }

    /// Returns `true` if this element should receive `$$definedVars` style injection.
    fn should_inject_define_vars(&self, name: &str) -> bool {
        !self.define_vars_values.is_empty() && css_scoping::should_scope_element(name)
    }

    /// Print all HTML attributes for an element, handling transition directives,
    /// `class`/`class:list` merging, spread attributes, and optional scope injection.
    ///
    /// `scope_id` contains the scope identifier when the element should be scoped.
    pub(super) fn print_html_attributes(
        &mut self,
        attrs: &[JSXAttributeItem<'a>],
        scope_id: Option<&ScopeId>,
        inject_define_vars: bool,
    ) {
        // Check for class + class:list combination that needs merging
        let mut static_class: Option<&str> = None;
        let mut class_list_expr: Option<&JSXExpressionContainer<'a>> = None;

        // Check for transition attributes
        let mut transition_name: Option<(String, oxc_span::Span)> = None;
        let mut transition_animate: Option<(String, oxc_span::Span)> = None;
        let mut transition_persist: Option<(String, oxc_span::Span)> = None;

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
                    transition_name = Some((Self::get_attr_value_string(attr), attr.span));
                } else if name == "transition:animate" {
                    transition_animate = Some((Self::get_attr_value_string(attr), attr.span));
                } else if name == "transition:persist" {
                    transition_persist =
                        Some((Self::get_attr_value_string_or_empty(attr), attr.span));
                }
            }
        }

        // If both class and class:list exist, merge them
        let has_merged_class = static_class.is_some() && class_list_expr.is_some();

        // Handle transition:persist — if there's a transition:name, use that for persist value.
        // Otherwise use $$createTransitionScope.
        if let Some((ref _persist_val, persist_span)) = transition_persist {
            self.add_source_mapping_for_span(persist_span);
            if let Some((ref name_val, _)) = transition_name {
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
            // Map to whichever transition attribute comes first
            if let Some((_, span)) = &transition_name {
                self.add_source_mapping_for_span(*span);
            } else if let Some((_, span)) = &transition_animate {
                self.add_source_mapping_for_span(*span);
            }
            let name_val = transition_name.map_or_else(|| "\"\"".to_string(), |(v, _)| v);
            let animate_val = transition_animate.map_or_else(|| "\"\"".to_string(), |(v, _)| v);
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

        // Track whether the scope class was already injected into an existing class/class:list
        let mut scope_injected = false;
        let mut has_class_attr = false;
        // Track whether $$definedVars style injection was already handled
        let mut define_vars_style_injected = false;

        // Pre-scan for class/class:list attributes
        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                if name == "class" || name == "class:list" {
                    has_class_attr = true;
                    break;
                }
            }
        }

        for attr in attrs {
            match attr {
                JSXAttributeItem::Attribute(attr) => {
                    let name = get_jsx_attribute_name(&attr.name);
                    // Skip set:html and set:text — handled separately
                    if name == "set:html" || name == "set:text" {
                        continue;
                    }
                    // Skip define:vars — Astro directive, not an HTML attribute
                    if name == "define:vars" {
                        continue;
                    }
                    // Skip is:global — Astro directive, not an HTML attribute
                    if name == "is:global" {
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
                    // Skip transition directives — already handled above.
                    // Exception: transition:persist-props is a simple rename, handled below.
                    if name.starts_with("transition:") && name != "transition:persist-props" {
                        continue;
                    }
                    // Skip individual class if we're merging with class:list
                    if has_merged_class && name == "class" {
                        continue;
                    }
                    // Handle `style` attribute with define:vars injection
                    if name == "style" && inject_define_vars {
                        self.print_style_with_define_vars(attr);
                        define_vars_style_injected = true;
                        self.define_vars_injected = true;
                        continue;
                    }
                    // Handle merged class:list (with scope class injection)
                    if has_merged_class && name == "class:list" {
                        if let (Some(static_val), Some(expr)) = (static_class, class_list_expr) {
                            self.add_source_mapping_for_span(attr.span);
                            self.print(&format!("${{{}([", runtime::ADD_ATTRIBUTE));
                            // Inject scope class into static_val if needed
                            if let Some(sid) = scope_id {
                                if sid.is_attribute_strategy() {
                                    // For attribute strategy, don't merge into class
                                    self.print(&format!(
                                        "\"{}\"",
                                        escape_double_quotes(static_val)
                                    ));
                                } else {
                                    self.print(&format!(
                                        "\"{} {}\"",
                                        escape_double_quotes(static_val),
                                        sid.class_value()
                                    ));
                                    scope_injected = true;
                                }
                            } else {
                                self.print(&format!("\"{}\"", escape_double_quotes(static_val)));
                            }
                            self.print(", ");
                            self.print_jsx_expression(&expr.expression);
                            self.print("], \"class:list\")}");
                        }
                        continue;
                    }
                    // Handle class attribute with scope class injection
                    if let Some(sid) = scope_id
                        && !sid.is_attribute_strategy()
                        && name == "class"
                    {
                        self.print_html_attribute_with_scope(attr, &sid.class_value());
                        scope_injected = true;
                        continue;
                    }
                    // Handle class:list attribute with scope class injection
                    if let Some(sid) = scope_id
                        && !sid.is_attribute_strategy()
                        && name == "class:list"
                    {
                        self.print_class_list_with_scope(attr, &sid.class_value());
                        scope_injected = true;
                        continue;
                    }
                    // transition:persist-props → data-astro-transition-persist-props
                    // Simple key rename, like the Go compiler does.
                    if name == "transition:persist-props" {
                        self.print_html_attribute_with_name(
                            attr,
                            "data-astro-transition-persist-props",
                        );
                        continue;
                    }
                    self.print_html_attribute(attr);
                }
                JSXAttributeItem::SpreadAttribute(spread) => {
                    self.add_source_mapping_for_span(spread.span);
                    // If we have a scope identifier and no explicit class/class:list,
                    // pass it as the 3rd argument to $$spreadAttributes
                    if let Some(sid) = scope_id
                        && !has_class_attr
                        && !scope_injected
                    {
                        self.print(&format!("${{{}(", runtime::SPREAD_ATTRIBUTES));
                        self.print_expression(&spread.argument);
                        // Always pass the class through $$spreadAttributes for runtime
                        // merging, regardless of scoped style strategy. The runtime's
                        // spreadAttributes only handles { class: ... } — it doesn't
                        // process arbitrary data attributes.
                        // For attribute strategy, the data-astro-cid-* attribute is
                        // added directly on the element by the fallback below.
                        let sc = sid.class_value();
                        self.print(&format!(",undefined,{{\"class\":\"{sc}\"}})}}"));
                        // Note: do NOT set scope_injected here for attribute strategy,
                        // so the data-astro-cid-* attribute is still added directly
                        // on the element by the fallback at the end of this function.
                        if !sid.is_attribute_strategy() {
                            scope_injected = true;
                        }
                        continue;
                    }
                    self.print(&format!("${{{}(", runtime::SPREAD_ATTRIBUTES));
                    self.print_expression(&spread.argument);
                    self.print(")}");
                }
            }
        }

        // If define:vars injection is needed but no `style` attribute existed, add one
        if inject_define_vars && !define_vars_style_injected {
            self.print(&format!(
                "${{{}($$definedVars, \"style\")}}",
                runtime::ADD_ATTRIBUTE
            ));
            self.define_vars_injected = true;
        }

        // If scope wasn't injected into any existing attribute, add it
        if let Some(sid) = scope_id
            && !scope_injected
        {
            if sid.is_attribute_strategy() {
                self.print(&format!(" {}", sid.data_attr_name()));
            } else {
                self.print(&format!(" class=\"{}\"", sid.class_value()));
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
                    let source = expr_to_string(e);
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

    /// Print a `style` attribute merged with `$$definedVars`.
    ///
    /// Handles all attribute value types following the Go compiler's `injectDefineVars()` logic:
    /// - Empty/boolean `style` → `${$$addAttribute($$definedVars, "style")}`
    /// - Quoted `style="val"` → `` ${$$addAttribute(`${"val"}; ${$$definedVars}`, "style")} ``
    /// - Expression `style={expr}` → if object `{...}` then `${$$addAttribute([{...},$$definedVars], "style")}`,
    ///   else `` ${$$addAttribute(`${expr}; ${$$definedVars}`, "style")} ``
    /// - Shorthand `{style}` → `` ${$$addAttribute(`${style}; ${$$definedVars}`, "style")} ``
    /// - Template literal `` style=`val` `` → `` ${$$addAttribute(`${`val`}; ${$$definedVars}`, "style")} ``
    fn print_style_with_define_vars(&mut self, attr: &JSXAttribute<'a>) {
        self.add_source_mapping_for_span(attr.span);
        match &attr.value {
            None => {
                // Empty/boolean style → $$definedVars
                self.print(&format!(
                    "${{{}($$definedVars, \"style\")}}",
                    runtime::ADD_ATTRIBUTE
                ));
            }
            Some(JSXAttributeValue::StringLiteral(lit)) => {
                // Quoted style="val" → `${"val"}; ${$$definedVars}`
                let val = lit.value.as_str();
                self.print(&format!(
                    "${{{}(`${{\"{}\"}}; ${{$$definedVars}}`, \"style\")}}",
                    runtime::ADD_ATTRIBUTE,
                    escape_double_quotes(val)
                ));
            }
            Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                if let Some(e) = expr.expression.as_expression() {
                    // Check if the expression is an ObjectExpression at the AST level
                    // (rather than relying on the string representation, which may be
                    // wrapped in parentheses by the codegen).
                    let is_object = matches!(e, Expression::ObjectExpression(_));
                    let expr_str = expr_to_string(e);
                    if is_object {
                        // Object expression: [{...},$$definedVars]
                        // Strip parentheses if present (oxc wraps objects in parens
                        // to avoid ambiguity with block statements).
                        let obj_str = expr_str
                            .trim()
                            .strip_prefix('(')
                            .and_then(|s| s.strip_suffix(')'))
                            .unwrap_or(expr_str.trim());
                        self.print(&format!(
                            "${{{}([{},$$definedVars], \"style\")}}",
                            runtime::ADD_ATTRIBUTE,
                            obj_str
                        ));
                    } else {
                        // Other expression: `${expr}; ${$$definedVars}`
                        self.print(&format!(
                            "${{{}(`${{{}}}; ${{$$definedVars}}`, \"style\")}}",
                            runtime::ADD_ATTRIBUTE,
                            expr_str
                        ));
                    }
                } else {
                    // Fallback: just $$definedVars
                    self.print(&format!(
                        "${{{}($$definedVars, \"style\")}}",
                        runtime::ADD_ATTRIBUTE
                    ));
                }
            }
            _ => {
                // Fallback: just $$definedVars
                self.print(&format!(
                    "${{{}($$definedVars, \"style\")}}",
                    runtime::ADD_ATTRIBUTE
                ));
            }
        }
    }

    /// Print a class attribute with scope class appended.
    fn print_html_attribute_with_scope(&mut self, attr: &JSXAttribute<'a>, scope_class: &str) {
        self.add_source_mapping_for_span(attr.span);
        match &attr.value {
            None => {
                // Empty class attribute → just the scope class
                self.print(&format!(" class=\"{scope_class}\""));
            }
            Some(JSXAttributeValue::StringLiteral(lit)) => {
                // Static class: append scope class
                let val = lit.value.as_str();
                if val.is_empty() {
                    self.print(&format!(" class=\"{scope_class}\""));
                } else {
                    self.print(&format!(
                        " class=\"{} {scope_class}\"",
                        escape_html_attribute(val)
                    ));
                }
            }
            Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                // Dynamic class: expression + scope class
                // Output: ${$$addAttribute((expr ?? "") + " astro-XXXX", "class")}
                self.print(&format!("${{{}((", runtime::ADD_ATTRIBUTE));
                self.print_jsx_expression(&expr.expression);
                self.print(&format!(" ?? \"\") + \" {scope_class}\", \"class\")}}"));
            }
            _ => {
                // Fallback: just output scope class
                self.print(&format!(" class=\"{scope_class}\""));
            }
        }
    }

    /// Print a class:list attribute with scope class appended.
    fn print_class_list_with_scope(&mut self, attr: &JSXAttribute<'a>, scope_class: &str) {
        self.add_source_mapping_for_span(attr.span);
        match &attr.value {
            Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                // class:list={expr} → ${$$addAttribute([expr, "astro-XXXX"], "class:list")}
                self.print(&format!("${{{}([(", runtime::ADD_ATTRIBUTE));
                self.print_jsx_expression(&expr.expression);
                self.print(&format!("), \"{scope_class}\"], \"class:list\")}}"));
            }
            _ => {
                // Fallback: just output scope class
                self.print(&format!(" class=\"{scope_class}\""));
            }
        }
    }

    /// Print a single HTML attribute (static or dynamic).
    fn print_html_attribute(&mut self, attr: &JSXAttribute<'a>) {
        let name = get_jsx_attribute_name(&attr.name);
        self.print_html_attribute_with_name(attr, &name);
    }

    /// Print a single HTML attribute using the given output name (for key renames).
    fn print_html_attribute_with_name(&mut self, attr: &JSXAttribute<'a>, name: &str) {
        self.add_source_mapping_for_span(attr.span);
        match &attr.value {
            None => {
                // Boolean attribute
                self.print(" ");
                self.print(name);
            }
            Some(value) => match value {
                JSXAttributeValue::StringLiteral(lit) => {
                    self.print(" ");
                    self.print(name);
                    self.print("=\"");
                    self.print(&escape_html_attribute(lit.value.as_str()));
                    self.print("\"");
                }
                JSXAttributeValue::ExpressionContainer(expr) => {
                    // Dynamic attribute
                    self.print(&format!("${{{}(", runtime::ADD_ATTRIBUTE));
                    self.print_jsx_expression(&expr.expression);
                    self.print(", \"");
                    self.print(name);
                    self.print("\")}");
                }
                JSXAttributeValue::Element(el) => {
                    // JSX element as attribute value (rare)
                    self.print(" ");
                    self.print(name);
                    self.print("=\"");
                    self.print_jsx_element(el);
                    self.print("\"");
                }
                JSXAttributeValue::Fragment(frag) => {
                    self.print(" ");
                    self.print(name);
                    self.print("=\"");
                    self.print_jsx_fragment(frag);
                    self.print("\"");
                }
            },
        }
    }
}
