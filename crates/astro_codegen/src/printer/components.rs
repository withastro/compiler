//! Component element printing and hydration handling.
//!
//! Contains `impl AstroCodegen` methods for rendering Astro/framework components
//! via `$$renderComponent`, including hydration directives (`client:load`,
//! `client:visible`, `client:only`, etc.) and `set:html`/`set:text` on components.

use super::AstroCodegen;
use super::elements::ScopeId;
use super::escape::{decode_html_entities, escape_double_quotes};
use super::expr_to_string;
use super::runtime;
use crate::css_scoping;
use crate::options::ScopedStyleStrategy;
use crate::scanner::{get_jsx_attribute_name, is_custom_element};
use oxc_ast::ast::*;

/// A client hydration directive parsed from a component's attributes.
pub(super) enum HydrationDirective {
    /// `client:only="framework"` — component is not rendered server-side.
    ClientOnly,
    /// Any other `client:*` directive (e.g. `load`, `idle`, `visible`, `media`).
    Other(String),
}

impl HydrationDirective {
    /// The directive name as it appears after `client:` (e.g. `"only"`, `"load"`).
    pub fn name(&self) -> &str {
        match self {
            Self::ClientOnly => "only",
            Self::Other(name) => name,
        }
    }

    pub fn is_client_only(&self) -> bool {
        matches!(self, Self::ClientOnly)
    }
}

/// Information about component hydration directives.
pub(super) struct HydrationInfo {
    /// The parsed hydration directive.
    pub directive: HydrationDirective,
    /// Component import path (for hydration).
    pub component_path: Option<String>,
    /// Component export name (for hydration).
    pub component_export: Option<String>,
}

/// Information about a `server:defer` directive on a component.
pub(super) struct ServerDeferInfo {
    /// Resolved component import path.
    pub component_path: Option<String>,
    /// Resolved component export name.
    pub component_export: Option<String>,
}

impl<'a> AstroCodegen<'a> {
    /// Check whether a component has a `server:defer` directive.
    pub(super) fn has_server_defer(attrs: &[JSXAttributeItem<'a>]) -> bool {
        attrs.iter().any(|attr| {
            if let JSXAttributeItem::Attribute(attr) = attr {
                get_jsx_attribute_name(&attr.name) == "server:defer"
            } else {
                false
            }
        })
    }

    /// Extract hydration info from a component's attributes.
    ///
    /// Returns `None` if the component has no `client:*` directive.
    pub(super) fn extract_hydration_info(attrs: &[JSXAttributeItem<'a>]) -> Option<HydrationInfo> {
        let mut directive = None;

        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);

                if let Some(d) = name.strip_prefix("client:") {
                    directive = Some(if d == "only" {
                        HydrationDirective::ClientOnly
                    } else {
                        HydrationDirective::Other(d.to_string())
                    });
                }
            }
        }

        Some(HydrationInfo {
            directive: directive?,
            component_path: None,
            component_export: None,
        })
    }

    /// Print a component element via `$$renderComponent`.
    pub(super) fn print_component_element(&mut self, el: &JSXElement<'a>, name: &str) {
        self.add_source_mapping_for_span(el.opening_element.span);
        // Check for client:* directives
        let mut hydration_info = Self::extract_hydration_info(&el.opening_element.attributes);

        // Check if this is a custom element (has dash in name)
        let is_custom = is_custom_element(name);

        // Check for server:defer directive
        let mut server_defer_info = if Self::has_server_defer(&el.opening_element.attributes) {
            Some(ServerDeferInfo {
                component_path: None,
                component_export: None,
            })
        } else {
            None
        };

        // Resolve component path and export for client:* hydrated components
        // This info is used for client:component-path and client:component-export attributes
        if let Some(info) = &mut hydration_info {
            // Handle member expressions like "components.A" or "defaultImport.Counter1"
            if name.contains('.') {
                let parts: Vec<&str> = name.split('.').collect();
                let namespace = parts[0];
                let property = parts[1..].join(".");

                if let Some(import_info) = self.component_imports.get(namespace) {
                    info.component_path = Some(import_info.specifier.clone());
                    // For namespace imports (import * as x), the export is just the property name
                    // For default imports (import x from), the export is "default.Property"
                    if import_info.is_namespace {
                        info.component_export = Some(property);
                    } else {
                        // Default or named import - prepend the original export name
                        info.component_export =
                            Some(format!("{}.{}", import_info.export_name, property));
                    }
                }
            } else if let Some(import_info) = self.component_imports.get(name) {
                info.component_path = Some(import_info.specifier.clone());
                info.component_export = Some(import_info.export_name.clone());
            }
        }

        // Resolve component path and export for server:defer components.
        // Use resolve_specifier to match the Go compiler's ResolveIdForMatch behaviour —
        // the resolved path must match the key used in serverIslandNameMap.
        if let Some(info) = &mut server_defer_info {
            if name.contains('.') {
                let parts: Vec<&str> = name.split('.').collect();
                let namespace = parts[0];
                let property = parts[1..].join(".");
                if let Some(import_info) = self.component_imports.get(namespace) {
                    info.component_path =
                        Some(self.options.resolve_specifier(&import_info.specifier));
                    info.component_export = if import_info.is_namespace {
                        Some(property)
                    } else {
                        Some(format!("{}.{}", import_info.export_name, property))
                    };
                }
            } else if let Some(import_info) = self.component_imports.get(name) {
                info.component_path = Some(self.options.resolve_specifier(&import_info.specifier));
                info.component_export = Some(import_info.export_name.clone());
            }
        }

        // Check for set:html or set:text on components (including Fragment)
        let set_directive = Self::extract_set_html_value(&el.opening_element.attributes);

        self.print("${");
        self.print(runtime::RENDER_COMPONENT);
        self.print("(");
        self.print(runtime::RESULT);
        self.print(",\"");
        self.print(name);
        self.print("\",");

        // Component reference - for client:only it's null, for custom elements it's
        // a quoted string, otherwise it's the component identifier
        if hydration_info
            .as_ref()
            .is_some_and(|h| h.directive.is_client_only())
        {
            self.print("null");
        } else if is_custom {
            // Custom elements use quoted tag name: "my-element"
            self.print("\"");
            self.print(name);
            self.print("\"");
        } else {
            self.print(name);
        }

        self.print(",{");

        // Determine if this component should receive a scope identifier.
        // Like the Go compiler, inject scope into all components (PascalCase and custom elements)
        // that are not in the NeverScopedElements list.
        let scope_id = if self.has_scoped_styles && css_scoping::should_scope_element(name) {
            let hash = &self.source_hash;
            match self.options.scoped_style_strategy() {
                ScopedStyleStrategy::Attribute => Some(ScopeId::DataAttribute(hash.clone())),
                _ => Some(ScopeId::Class(format!("astro-{hash}"))),
            }
        } else {
            None
        };

        // Components always receive slot as a prop.
        // Only HTML elements have the slot attribute stripped when inside named slots.
        let prev_skip_slot = self.skip_slot_attribute;
        self.skip_slot_attribute = false;

        // Print attributes as object properties (skip set:html/set:text if present)
        self.print_component_attributes_filtered(
            &el.opening_element.attributes,
            hydration_info.as_ref(),
            server_defer_info.as_ref(),
            if set_directive.is_some() {
                Some(&["set:html", "set:text"])
            } else {
                None
            },
            scope_id.as_ref(),
        );

        self.skip_slot_attribute = prev_skip_slot;

        self.print("}");

        // For set:html or set:text, create a default slot with the content
        if let Some((value, is_html, needs_unescape, is_raw_text, set_span)) = set_directive {
            self.add_source_mapping_for_span(set_span);
            let async_prefix = self.get_async_prefix();
            let slot_params = self.get_slot_params();
            self.print(&format!(",{{\"default\": {async_prefix}{slot_params}"));
            self.print(runtime::RENDER);
            self.print("`");
            if is_raw_text {
                // set:text with string literal - inline raw text without ${}
                self.print(&value);
            } else {
                self.print("${");
                if is_html && needs_unescape {
                    // set:html with expression needs $$unescapeHTML
                    self.print(runtime::UNESCAPE_HTML);
                    self.print("(");
                    self.print(&value);
                    self.print(")");
                } else {
                    // set:html with string literal or set:text with expression - just interpolate directly
                    self.print(&value);
                }
                self.print("}");
            }
            self.print("`,}");
        } else if !el.children.is_empty() {
            // Print slots if there are children
            self.print(",");
            // Custom elements use browser's Shadow DOM slots, not Astro slots
            // All children go to default slot with their slot attributes preserved
            if is_custom {
                self.print_component_default_slot_only(&el.children);
            } else {
                self.print_component_slots(&el.children);
            }
        }

        // Map the closing tag (e.g. </Card>) to the `)` that closes
        // $$renderComponent(...) — the semantic equivalent in generated code.
        if let Some(ref closing) = el.closing_element {
            self.add_source_mapping_for_span(closing.span);
        }
        self.print(")}");
    }

    /// Extract `set:html` or `set:text` value from component attributes.
    ///
    /// Returns `(value_string, is_html, needs_unescape, is_raw_text, span)`:
    /// - `is_html` is `true` for `set:html`, `false` for `set:text`
    /// - `needs_unescape` is `true` for expressions (need `$$unescapeHTML`), `false` for literals
    /// - `is_raw_text` is `true` for `set:text` with string literal (should be inlined without `${}`)
    pub(super) fn extract_set_html_value(
        attrs: &[JSXAttributeItem<'a>],
    ) -> Option<(String, bool, bool, bool, oxc_span::Span)> {
        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                let is_html = name == "set:html";
                let is_text = name == "set:text";
                if is_html || is_text {
                    let (value, needs_unescape, is_raw_text) = match &attr.value {
                        Some(JSXAttributeValue::StringLiteral(lit)) => {
                            // String literals don't need $$unescapeHTML
                            let raw_value = lit.value.as_str();
                            if is_html {
                                let decoded = decode_html_entities(raw_value);
                                (
                                    Some(format!("\"{}\"", escape_double_quotes(&decoded))),
                                    false,
                                    false,
                                )
                            } else {
                                // set:text with string literal - return raw value for inline
                                (Some(raw_value.to_string()), false, true)
                            }
                        }
                        Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                            if let Some(e) = expr.expression.as_expression() {
                                // set:html needs $$unescapeHTML for expressions, but NOT for
                                // template literals — the Go compiler passes template literals
                                // through as-is without unescaping.
                                let is_template_literal =
                                    matches!(e, Expression::TemplateLiteral(_));
                                let needs_unescape = is_html && !is_template_literal;
                                let code = expr_to_string(e);
                                return Some((code, is_html, needs_unescape, false, attr.span));
                            }
                            (None, true, false)
                        }
                        _ => (None, false, false),
                    };
                    return value.map(|v| (v, is_html, needs_unescape, is_raw_text, attr.span));
                }
            }
        }
        None
    }

    /// Print component attributes, optionally filtering out certain names.
    pub(super) fn print_component_attributes_filtered(
        &mut self,
        attrs: &[JSXAttributeItem<'a>],
        hydration: Option<&HydrationInfo>,
        server_defer: Option<&ServerDeferInfo>,
        skip_names: Option<&[&str]>,
        scope_id: Option<&ScopeId>,
    ) {
        let mut first = true;

        // Pre-scan for transition attributes
        let mut transition_name: Option<(String, oxc_span::Span)> = None;
        let mut transition_animate: Option<(String, oxc_span::Span)> = None;
        let mut transition_persist: Option<oxc_span::Span> = None;
        let mut transition_persist_props: Option<(String, oxc_span::Span)> = None;

        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                if name == "transition:name" {
                    transition_name = Some((Self::get_attr_value_string(attr), attr.span));
                } else if name == "transition:animate" {
                    transition_animate = Some((Self::get_attr_value_string(attr), attr.span));
                } else if name == "transition:persist" {
                    transition_persist = Some(attr.span);
                } else if name == "transition:persist-props" {
                    transition_persist_props = Some((Self::get_attr_value_string(attr), attr.span));
                }
            }
        }

        // Track whether the scope class was merged into an existing class attribute
        let mut scope_injected = false;

        // Determine the scope class string (for class/where strategy only)
        let scope_class = scope_id.and_then(|sid| {
            if sid.is_attribute_strategy() {
                None
            } else {
                Some(sid.class_value())
            }
        });

        // Print regular attributes first
        for attr in attrs {
            match attr {
                JSXAttributeItem::Attribute(attr) => {
                    let name = get_jsx_attribute_name(&attr.name);

                    // Skip slot attribute when skip_slot_attribute is true
                    if name == "slot" && self.skip_slot_attribute {
                        continue;
                    }

                    // Skip filtered names
                    if let Some(names) = skip_names
                        && names.contains(&name.as_str())
                    {
                        continue;
                    }

                    // Skip transition directives - already handled above
                    if name.starts_with("transition:") {
                        continue;
                    }

                    // Skip is:raw directive
                    if name == "is:raw" {
                        continue;
                    }

                    // Skip server:defer directive
                    if name == "server:defer" {
                        continue;
                    }

                    if !first {
                        self.print(",");
                    }
                    first = false;

                    self.add_source_mapping_for_span(attr.span);
                    self.print("\"");
                    self.print(&name);
                    self.print("\":");

                    // Merge scope class into class attribute value (matches Go compiler).
                    // Static:  class="foo" → "class":"foo astro-HASH"
                    // Dynamic: class={expr} → "class":(((expr) ?? "") + " astro-HASH")
                    // Boolean: class        → "class":"astro-HASH"
                    if name == "class"
                        && let Some(sc) = &scope_class
                    {
                        match &attr.value {
                            None => {
                                // Boolean class attribute → just the scope class
                                self.print(&format!("\"{sc}\""));
                            }
                            Some(JSXAttributeValue::StringLiteral(lit)) => {
                                let val = lit.value.as_str();
                                if val.is_empty() {
                                    self.print(&format!("\"{sc}\""));
                                } else {
                                    self.print(&format!("\"{} {sc}\"", escape_double_quotes(val)));
                                }
                            }
                            Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                                self.print("(((");
                                self.print_jsx_expression(&expr.expression);
                                self.print(&format!(") ?? \"\") + \" {sc}\")"));
                            }
                            _ => {
                                self.print(&format!("\"{sc}\""));
                            }
                        }
                        scope_injected = true;
                        continue;
                    }

                    match &attr.value {
                        None => {
                            self.print("true");
                        }
                        Some(JSXAttributeValue::StringLiteral(lit)) => {
                            self.print("\"");
                            self.print(&escape_double_quotes(lit.value.as_str()));
                            self.print("\"");
                        }
                        Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                            if let Some(
                                Expression::TemplateLiteral(_) | Expression::StringLiteral(_),
                            ) = expr.expression.as_expression()
                            {
                                self.print_jsx_expression(&expr.expression);
                            } else {
                                self.print("(");
                                self.print_jsx_expression(&expr.expression);
                                self.print(")");
                            }
                        }
                        Some(JSXAttributeValue::Element(_el)) => {
                            self.print("\"[JSX]\"");
                        }
                        Some(JSXAttributeValue::Fragment(_)) => {
                            self.print("\"[Fragment]\"");
                        }
                    }
                }
                JSXAttributeItem::SpreadAttribute(spread) => {
                    if !first {
                        self.print(",");
                    }
                    first = false;
                    self.add_source_mapping_for_span(spread.span);
                    self.print("...(");
                    self.print_expression(&spread.argument);
                    self.print(")");
                }
            }
        }

        // Print transition attributes AFTER regular attributes
        if transition_name.is_some() || transition_animate.is_some() {
            if !first {
                self.print(",");
            }
            first = false;
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
                "\"data-astro-transition-scope\":({}({}, \"{}\", {}, {}))",
                runtime::RENDER_TRANSITION,
                runtime::RESULT,
                hash,
                animate_val,
                name_val
            ));
        }

        // Print transition:persist-props as a data attribute if present
        if let Some((props_val, persist_props_span)) = &transition_persist_props {
            if !first {
                self.print(",");
            }
            first = false;
            self.add_source_mapping_for_span(*persist_props_span);
            self.print(&format!(
                "\"data-astro-transition-persist-props\":{props_val}"
            ));
        }

        if let Some(persist_span) = transition_persist {
            if !first {
                self.print(",");
            }
            first = false;
            self.add_source_mapping_for_span(persist_span);
            let hash = self.generate_transition_hash();
            self.print(&format!(
                "\"data-astro-transition-persist\":({}({}, \"{}\"))",
                runtime::CREATE_TRANSITION_SCOPE,
                runtime::RESULT,
                hash
            ));
        }

        // Add scope identifier as a prop if not already merged into an existing class attribute.
        // For attribute strategy: always add "data-astro-cid-HASH": true
        // For class/where strategy: add "class": "astro-HASH" only if no class attr existed
        if let Some(sid) = scope_id {
            if sid.is_attribute_strategy() {
                if !first {
                    self.print(",");
                }
                first = false;
                let attr_name = sid.data_attr_name();
                self.print(&format!("\"{attr_name}\":true"));
            } else if !scope_injected {
                if !first {
                    self.print(",");
                }
                first = false;
                let sc = sid.class_value();
                self.print(&format!("\"class\":\"{sc}\""));
            }
        }

        // Add hydration attributes if present
        if let Some(hydration) = hydration {
            if !first {
                self.print(",");
            }
            self.print(&format!(
                "\"client:component-hydration\":\"{}\"",
                hydration.directive.name()
            ));

            if let Some(path) = &hydration.component_path {
                if hydration.directive.is_client_only() && !self.options.has_resolve_path() {
                    self.print(&format!(
                        ",\"client:component-path\":($$metadata.resolvePath(\"{path}\"))"
                    ));
                } else {
                    self.print(&format!(",\"client:component-path\":(\"{path}\")"));
                }
            }

            if let Some(export) = &hydration.component_export {
                if hydration.directive.is_client_only() {
                    self.print(&format!(",\"client:component-export\":\"{export}\""));
                } else {
                    self.print(&format!(",\"client:component-export\":(\"{export}\")"));
                }
            }
        }

        // Add server:defer attributes if present — these signal the runtime to replace this
        // component with a server island placeholder instead of rendering it inline.
        if let Some(server_defer) = server_defer {
            if !first {
                self.print(",");
            }
            // Matches Go compiler: "server:component-directive": "defer"
            self.print("\"server:component-directive\":\"defer\"");

            if let Some(path) = &server_defer.component_path {
                self.print(&format!(",\"server:component-path\":(\"{path}\")"));
            }

            if let Some(export) = &server_defer.component_export {
                self.print(&format!(",\"server:component-export\":(\"{export}\")"));
            }
        }
    }
}
