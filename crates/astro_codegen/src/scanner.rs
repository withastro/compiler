//! Astro AST scanner.
//!
//! Pre-analyzes an `AstroRoot` AST in a single pass using the `Visit` trait
//! to collect metadata needed by the printer. This separates the analysis
//! phase from the code generation phase.

use oxc_ast::ast::*;
use oxc_ast_visit::{Visit, walk};
use oxc_codegen::Codegen;
use rustc_hash::FxHashSet;

use oxc_allocator::Allocator;

/// Information about an imported component.
#[derive(Debug, Clone)]
#[expect(dead_code)]
pub struct ComponentImportInfo {
    /// The import specifier (e.g., "../components")
    pub specifier: String,
    /// The export name ("default" for default imports, otherwise the named export)
    pub export_name: String,
    /// Whether this is a namespace import (import * as x)
    pub is_namespace: bool,
}

/// Type of hoisted script (internal representation).
#[derive(Debug, Clone)]
pub enum HoistedScriptType {
    /// Inline script: `{ type: "inline", value: \`...\` }`
    Inline,
    /// External script with src: `{ type: "external", src: '...' }`
    External,
    /// Script with define:vars: `{ type: "define:vars", value: \`...\`, keys: '...' }`
    DefineVars,
}

/// Information about a hoisted script (internal representation).
#[derive(Debug, Clone)]
pub struct HoistedScript {
    pub script_type: HoistedScriptType,
    /// The script content (for inline and define:vars)
    pub value: Option<String>,
    /// The external src (for external scripts)
    pub src: Option<String>,
    /// The variable keys (for define:vars)
    pub keys: Option<String>,
}

/// Information about a hydrated component.
#[derive(Debug, Clone)]
pub struct HydratedComponent {
    /// The component name (e.g., "One", "my-element")
    pub name: String,
    /// Whether this is a custom element (has a dash in the name)
    pub is_custom_element: bool,
}

/// Result of scanning an Astro AST.
///
/// Contains all metadata collected during the analysis pass.
/// The printer consumes this without doing any further analysis.
#[derive(Debug)]
pub struct ScanResult {
    /// Whether the source uses the `Astro` global (e.g., `Astro.props`)
    pub uses_astro_global: bool,
    /// Whether the source uses transition directives (`transition:*`, `server:defer`)
    pub uses_transitions: bool,
    /// Whether the source contains an `await` expression (needs async wrappers)
    pub has_await: bool,
    /// Set of component names (or namespace roots) that use `client:only`.
    /// Used by the printer to detect client:only imports during frontmatter processing.
    pub client_only_component_names: FxHashSet<String>,
    /// Collected hydrated components for metadata
    pub hydrated_components: Vec<HydratedComponent>,
    /// Collected client-only components with full names (including dot-notation)
    pub client_only_components: Vec<HydratedComponent>,
    /// Collected server-deferred components (with `server:defer` directive)
    pub server_deferred_components: Vec<HydratedComponent>,
    /// Collected hydration directive names (e.g., "load", "visible", "only")
    pub hydration_directives: Vec<String>,
    /// Collected hoisted scripts
    pub hoisted_scripts: Vec<HoistedScript>,
}

/// Scans an Astro AST to collect metadata in a single pass.
///
/// Uses the `Visit` trait to walk the entire tree once, detecting:
/// - `Astro` global usage (in frontmatter and template expressions)
/// - `transition:*` / `server:defer` directives
/// - `client:*` hydration directives and component tracking
/// - Hoisted `<script>` elements
pub struct AstroScanner<'a> {
    allocator: &'a Allocator,
    /// Whether we've found an `Astro` identifier reference
    uses_astro_global: bool,
    /// Whether we've found transition directives
    uses_transitions: bool,
    /// Whether we've found an `await` expression
    has_await: bool,
    /// Set of component names that use `client:only`
    client_only_component_names: FxHashSet<String>,
    /// Collected hydrated components
    hydrated_components: Vec<HydratedComponent>,
    /// Collected client-only components (full names including dot-notation)
    client_only_components: Vec<HydratedComponent>,
    /// Collected server-deferred components
    server_deferred_components: Vec<HydratedComponent>,
    /// Hydration directive names
    hydration_directives: Vec<String>,
    /// Collected hoisted scripts
    hoisted_scripts: Vec<HoistedScript>,
}

impl<'a> AstroScanner<'a> {
    pub fn new(allocator: &'a Allocator) -> Self {
        Self {
            allocator,
            uses_astro_global: false,
            uses_transitions: false,
            has_await: false,
            client_only_component_names: FxHashSet::default(),
            hydrated_components: Vec::new(),
            client_only_components: Vec::new(),
            server_deferred_components: Vec::new(),
            hydration_directives: Vec::new(),
            hoisted_scripts: Vec::new(),
        }
    }

    /// Run the scanner on an Astro AST and return the collected metadata.
    pub fn scan(mut self, root: &AstroRoot<'a>) -> ScanResult {
        self.visit_astro_root(root);
        ScanResult {
            uses_astro_global: self.uses_astro_global,
            uses_transitions: self.uses_transitions,
            has_await: self.has_await,
            client_only_component_names: self.client_only_component_names,
            hydrated_components: self.hydrated_components,
            client_only_components: self.client_only_components,
            server_deferred_components: self.server_deferred_components,
            hydration_directives: self.hydration_directives,
            hoisted_scripts: self.hoisted_scripts,
        }
    }

    /// Process a JSX opening element for client:* and transition:* directives.
    fn process_element_directives(&mut self, el: &JSXOpeningElement<'a>) {
        let name = get_jsx_element_name(&el.name);
        let is_component = is_component_name(&name);
        let is_custom = is_custom_element(&name);

        for attr in &el.attributes {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let attr_name = get_jsx_attribute_name(&attr.name);

                // Detect transition directives
                if attr_name.starts_with("transition:") || attr_name == "server:defer" {
                    self.uses_transitions = true;
                }

                // Detect server:defer components
                if attr_name == "server:defer"
                    && (is_component || is_custom)
                    && !self.server_deferred_components.iter().any(|c| c.name == name)
                {
                    self.server_deferred_components.push(HydratedComponent {
                        name: name.clone(),
                        is_custom_element: is_custom,
                    });
                }

                // Detect client:* directives
                if let Some(directive) = attr_name.strip_prefix("client:") {
                    if directive == "only" {
                        // Store the namespace root (or simple name) for import-level checks
                        if name.contains('.') {
                            if let Some(namespace) = name.split('.').next() {
                                self.client_only_component_names.insert(namespace.to_string());
                            }
                        } else {
                            self.client_only_component_names.insert(name.clone());
                        }
                        // Store the full component name for metadata resolution
                        if (is_component || is_custom)
                            && !self.client_only_components.iter().any(|c| c.name == name)
                        {
                            self.client_only_components.push(HydratedComponent {
                                name,
                                is_custom_element: is_custom,
                            });
                        }
                        if !self.hydration_directives.contains(&"only".to_string()) {
                            self.hydration_directives.push("only".to_string());
                        }
                    } else {
                        if !self.hydration_directives.contains(&directive.to_string()) {
                            self.hydration_directives.push(directive.to_string());
                        }
                        if (is_component || is_custom)
                            && !self.hydrated_components.iter().any(|c| c.name == name)
                        {
                            self.hydrated_components
                                .push(HydratedComponent { name, is_custom_element: is_custom });
                        }
                    }
                    break; // Only process first client:* directive
                }
            }
        }
    }

    /// Check if a <script> element should be hoisted and collect it.
    fn try_collect_script(&mut self, el: &JSXElement<'a>) {
        let name = get_jsx_element_name(&el.opening_element.name);
        if name != "script" {
            return;
        }
        if !should_hoist_script(&el.opening_element.attributes) {
            return;
        }

        let attrs: Vec<_> = el.opening_element.attributes.iter().collect();

        // Try AstroScript child first (parsed JS/TS content)
        for child in &el.children {
            if let JSXChild::AstroScript(script) = child {
                self.collect_script_from_program(&script.program, &attrs);
                return;
            }
        }

        // Fall back to JSXText child (raw text content)
        self.collect_script_from_text_children(&el.children, &attrs);
    }

    /// Collect a script from its parsed program and attributes.
    fn collect_script_from_program(
        &mut self,
        program: &Program<'a>,
        attrs: &[&JSXAttributeItem<'a>],
    ) {
        let mut src_value: Option<String> = None;
        let mut define_vars_value: Option<String> = None;
        let mut define_vars_keys: Option<String> = None;

        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let attr_name = get_jsx_attribute_name(&attr.name);
                if attr_name == "src" {
                    if let Some(JSXAttributeValue::StringLiteral(lit)) = &attr.value {
                        src_value = Some(lit.value.to_string());
                    }
                } else if attr_name == "define:vars"
                    && let Some(JSXAttributeValue::ExpressionContainer(container)) = &attr.value
                    && let Some(expr) = container.expression.as_expression()
                    && let Expression::ObjectExpression(obj) = expr
                {
                    let keys: Vec<String> = obj
                        .properties
                        .iter()
                        .filter_map(|prop| {
                            if let ObjectPropertyKind::ObjectProperty(p) = prop {
                                Some(get_property_key_name(&p.key))
                            } else {
                                None
                            }
                        })
                        .collect();
                    define_vars_keys = Some(keys.join(","));
                    define_vars_value = Some(get_script_content(self.allocator, program));
                }
            }
        }

        if let Some(keys) = define_vars_keys {
            self.hoisted_scripts.push(HoistedScript {
                script_type: HoistedScriptType::DefineVars,
                value: Some(define_vars_value.unwrap_or_default()),
                src: None,
                keys: Some(keys),
            });
        } else if let Some(src) = src_value {
            self.hoisted_scripts.push(HoistedScript {
                script_type: HoistedScriptType::External,
                value: None,
                src: Some(src),
                keys: None,
            });
        } else {
            let content = get_script_content(self.allocator, program);
            if !content.is_empty() {
                self.hoisted_scripts.push(HoistedScript {
                    script_type: HoistedScriptType::Inline,
                    value: Some(content),
                    src: None,
                    keys: None,
                });
            }
        }
    }

    /// Collect a script from JSXText children (raw text content).
    fn collect_script_from_text_children(
        &mut self,
        children: &[JSXChild<'a>],
        attrs: &[&JSXAttributeItem<'a>],
    ) {
        let mut src_value: Option<String> = None;

        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let attr_name = get_jsx_attribute_name(&attr.name);
                if attr_name == "src"
                    && let Some(JSXAttributeValue::StringLiteral(lit)) = &attr.value
                {
                    src_value = Some(lit.value.to_string());
                }
            }
        }

        if let Some(src) = src_value {
            self.hoisted_scripts.push(HoistedScript {
                script_type: HoistedScriptType::External,
                value: None,
                src: Some(src),
                keys: None,
            });
        } else {
            let content: String = children
                .iter()
                .filter_map(|child| {
                    if let JSXChild::Text(text) = child { Some(text.value.as_str()) } else { None }
                })
                .collect::<Vec<_>>()
                .join("");

            let content = content.trim();
            if !content.is_empty() {
                self.hoisted_scripts.push(HoistedScript {
                    script_type: HoistedScriptType::Inline,
                    value: Some(content.to_string()),
                    src: None,
                    keys: None,
                });
            }
        }
    }
}

impl<'a> Visit<'a> for AstroScanner<'a> {
    /// Detect `Astro` identifier references in frontmatter and template expressions.
    fn visit_identifier_reference(&mut self, ident: &IdentifierReference<'a>) {
        if ident.name == "Astro" {
            self.uses_astro_global = true;
        }
    }

    /// Process JSX elements for directives and script hoisting.
    fn visit_jsx_element(&mut self, el: &JSXElement<'a>) {
        // Check for directives on the opening element
        self.process_element_directives(&el.opening_element);

        // Check for hoistable scripts — if we collect one, skip walking
        // children to avoid visit_astro_script double-collecting the same script.
        let name = get_jsx_element_name(&el.opening_element.name);
        if name == "script" && should_hoist_script(&el.opening_element.attributes) {
            self.try_collect_script(el);
            // Don't walk children — we already processed the AstroScript child
            return;
        }

        // Continue walking children (the default walk handles this)
        walk::walk_jsx_element(self, el);
    }

    /// Detect `await` expressions — used to determine if async wrappers are needed.
    fn visit_await_expression(&mut self, it: &AwaitExpression<'a>) {
        self.has_await = true;
        walk::walk_await_expression(self, it);
    }

    /// Process standalone AstroScript nodes (direct children of the root,
    /// not inside a `<script>` JSXElement — those are handled by visit_jsx_element).
    fn visit_astro_script(&mut self, script: &AstroScript<'a>) {
        self.collect_script_from_program(&script.program, &[]);
        // Don't walk into the program — we've already handled it
    }
}

// --- Helper functions (shared with printer) ---

pub fn get_jsx_element_name(name: &JSXElementName<'_>) -> String {
    match name {
        JSXElementName::Identifier(ident) => ident.name.to_string(),
        JSXElementName::IdentifierReference(ident) => ident.name.to_string(),
        JSXElementName::NamespacedName(ns) => {
            format!("{}:{}", ns.namespace.name, ns.name.name)
        }
        JSXElementName::MemberExpression(expr) => get_jsx_member_expression_name(expr),
        JSXElementName::ThisExpression(_) => "this".to_string(),
    }
}

fn get_jsx_member_expression_name(expr: &JSXMemberExpression<'_>) -> String {
    let object_name = match &expr.object {
        JSXMemberExpressionObject::IdentifierReference(ident) => ident.name.to_string(),
        JSXMemberExpressionObject::MemberExpression(inner) => get_jsx_member_expression_name(inner),
        JSXMemberExpressionObject::ThisExpression(_) => "this".to_string(),
    };
    format!("{object_name}.{}", expr.property.name)
}

pub fn get_jsx_attribute_name(name: &JSXAttributeName<'_>) -> String {
    match name {
        JSXAttributeName::Identifier(ident) => ident.name.to_string(),
        JSXAttributeName::NamespacedName(ns) => {
            format!("{}:{}", ns.namespace.name, ns.name.name)
        }
    }
}

pub fn is_component_name(name: &str) -> bool {
    name.starts_with(|c: char| c.is_ascii_uppercase()) || name.contains('.') || name.contains(':')
}

pub fn is_custom_element(name: &str) -> bool {
    name.contains('-')
}

pub fn should_hoist_script(attrs: &oxc_allocator::Vec<'_, JSXAttributeItem<'_>>) -> bool {
    let mut has_hoist = false;
    let mut has_type_module = false;
    let mut is_inline = false;

    for attr in attrs {
        if let JSXAttributeItem::Attribute(attr) = attr {
            let attr_name = get_jsx_attribute_name(&attr.name);
            match attr_name.as_str() {
                "hoist" => has_hoist = true,
                "is:inline" => is_inline = true,
                "define:vars" => return true,
                "type" => {
                    if let Some(JSXAttributeValue::StringLiteral(lit)) = &attr.value
                        && lit.value == "module"
                    {
                        has_type_module = true;
                    }
                }
                _ => {}
            }
        }
    }

    if is_inline {
        return false;
    }

    // Scripts with no attributes at all, or with just `type="module"`, or with `hoist`
    // are hoistable. This matches the Go compiler's IsHoistable logic.
    attrs.is_empty() || has_hoist || has_type_module || no_meaningful_attrs(attrs)
}

fn no_meaningful_attrs(attrs: &oxc_allocator::Vec<'_, JSXAttributeItem<'_>>) -> bool {
    for attr in attrs {
        if let JSXAttributeItem::Attribute(attr) = attr {
            let name = get_jsx_attribute_name(&attr.name);
            match name.as_str() {
                "src" => return true, // src-only scripts are hoistable
                "type" | "hoist" | "is:inline" | "define:vars" => {}
                _ => return false,
            }
        }
    }
    true
}

fn get_property_key_name(key: &PropertyKey<'_>) -> String {
    match key {
        PropertyKey::StaticIdentifier(ident) => ident.name.to_string(),
        PropertyKey::StringLiteral(lit) => lit.value.to_string(),
        _ => String::new(),
    }
}

/// Get the script content as a string by codegen-ing the program.
fn get_script_content(_allocator: &Allocator, program: &Program<'_>) -> String {
    let codegen = Codegen::new();
    let output = codegen.build(program);
    output.code.trim_end().to_string()
}
