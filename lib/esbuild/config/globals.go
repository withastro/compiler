package config

import (
	"sync"
)

var processedGlobalsMutex sync.Mutex

// If something is in this list, then a direct identifier expression or property
// access chain matching this will be assumed to have no side effects and will
// be removed.
//
// This also means code is allowed to be reordered past things in this list. For
// example, if "console.log" is in this list, permitting reordering allows for
// "if (a) console.log(b); else console.log(c)" to be reordered and transformed
// into "console.log(a ? b : c)". Notice that "a" and "console.log" are in a
// different order, which can only happen if evaluating the "console.log"
// property access can be assumed to not change the value of "a".
//
// Note that membership in this list says nothing about whether calling any of
// these functions has any side effects. It only says something about
// referencing these function without calling them.
var knownGlobals = [][]string{
	// These global identifiers should exist in all JavaScript environments. This
	// deliberately omits "NaN", "Infinity", and "undefined" because these are
	// treated as automatically-inlined constants instead of identifiers.
	{"Array"},
	{"Boolean"},
	{"Function"},
	{"Math"},
	{"Number"},
	{"Object"},
	{"RegExp"},
	{"String"},

	// Object: Static methods
	// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Object#Static_methods
	{"Object", "assign"},
	{"Object", "create"},
	{"Object", "defineProperties"},
	{"Object", "defineProperty"},
	{"Object", "entries"},
	{"Object", "freeze"},
	{"Object", "fromEntries"},
	{"Object", "getOwnPropertyDescriptor"},
	{"Object", "getOwnPropertyDescriptors"},
	{"Object", "getOwnPropertyNames"},
	{"Object", "getOwnPropertySymbols"},
	{"Object", "getPrototypeOf"},
	{"Object", "is"},
	{"Object", "isExtensible"},
	{"Object", "isFrozen"},
	{"Object", "isSealed"},
	{"Object", "keys"},
	{"Object", "preventExtensions"},
	{"Object", "seal"},
	{"Object", "setPrototypeOf"},
	{"Object", "values"},

	// Object: Instance methods
	// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Object#Instance_methods
	{"Object", "prototype", "__defineGetter__"},
	{"Object", "prototype", "__defineSetter__"},
	{"Object", "prototype", "__lookupGetter__"},
	{"Object", "prototype", "__lookupSetter__"},
	{"Object", "prototype", "hasOwnProperty"},
	{"Object", "prototype", "isPrototypeOf"},
	{"Object", "prototype", "propertyIsEnumerable"},
	{"Object", "prototype", "toLocaleString"},
	{"Object", "prototype", "toString"},
	{"Object", "prototype", "unwatch"},
	{"Object", "prototype", "valueOf"},
	{"Object", "prototype", "watch"},

	// Math: Static properties
	// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Math#Static_properties
	{"Math", "E"},
	{"Math", "LN10"},
	{"Math", "LN2"},
	{"Math", "LOG10E"},
	{"Math", "LOG2E"},
	{"Math", "PI"},
	{"Math", "SQRT1_2"},
	{"Math", "SQRT2"},

	// Math: Static methods
	// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Math#Static_methods
	{"Math", "abs"},
	{"Math", "acos"},
	{"Math", "acosh"},
	{"Math", "asin"},
	{"Math", "asinh"},
	{"Math", "atan"},
	{"Math", "atan2"},
	{"Math", "atanh"},
	{"Math", "cbrt"},
	{"Math", "ceil"},
	{"Math", "clz32"},
	{"Math", "cos"},
	{"Math", "cosh"},
	{"Math", "exp"},
	{"Math", "expm1"},
	{"Math", "floor"},
	{"Math", "fround"},
	{"Math", "hypot"},
	{"Math", "imul"},
	{"Math", "log"},
	{"Math", "log10"},
	{"Math", "log1p"},
	{"Math", "log2"},
	{"Math", "max"},
	{"Math", "min"},
	{"Math", "pow"},
	{"Math", "random"},
	{"Math", "round"},
	{"Math", "sign"},
	{"Math", "sin"},
	{"Math", "sinh"},
	{"Math", "sqrt"},
	{"Math", "tan"},
	{"Math", "tanh"},
	{"Math", "trunc"},

	// Reflect: Static methods
	// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Reflect#static_methods
	{"Reflect", "apply"},
	{"Reflect", "construct"},
	{"Reflect", "defineProperty"},
	{"Reflect", "deleteProperty"},
	{"Reflect", "get"},
	{"Reflect", "getOwnPropertyDescriptor"},
	{"Reflect", "getPrototypeOf"},
	{"Reflect", "has"},
	{"Reflect", "isExtensible"},
	{"Reflect", "ownKeys"},
	{"Reflect", "preventExtensions"},
	{"Reflect", "set"},
	{"Reflect", "setPrototypeOf"},

	// Other globals present in both the browser and node (except "eval" because
	// it has special behavior)
	{"AbortController"},
	{"AbortSignal"},
	{"AggregateError"},
	{"ArrayBuffer"},
	{"BigInt"},
	{"DataView"},
	{"Date"},
	{"Error"},
	{"EvalError"},
	{"Event"},
	{"EventTarget"},
	{"Float32Array"},
	{"Float64Array"},
	{"Int16Array"},
	{"Int32Array"},
	{"Int8Array"},
	{"Intl"},
	{"JSON"},
	{"Map"},
	{"MessageChannel"},
	{"MessageEvent"},
	{"MessagePort"},
	{"Promise"},
	{"Proxy"},
	{"RangeError"},
	{"ReferenceError"},
	{"Reflect"},
	{"Set"},
	{"Symbol"},
	{"SyntaxError"},
	{"TextDecoder"},
	{"TextEncoder"},
	{"TypeError"},
	{"URIError"},
	{"URL"},
	{"URLSearchParams"},
	{"Uint16Array"},
	{"Uint32Array"},
	{"Uint8Array"},
	{"Uint8ClampedArray"},
	{"WeakMap"},
	{"WeakSet"},
	{"WebAssembly"},
	{"clearInterval"},
	{"clearTimeout"},
	{"console"},
	{"decodeURI"},
	{"decodeURIComponent"},
	{"encodeURI"},
	{"encodeURIComponent"},
	{"escape"},
	{"globalThis"},
	{"isFinite"},
	{"isNaN"},
	{"parseFloat"},
	{"parseInt"},
	{"queueMicrotask"},
	{"setInterval"},
	{"setTimeout"},
	{"unescape"},

	// Console method references are assumed to have no side effects
	// https://developer.mozilla.org/en-US/docs/Web/API/console
	{"console", "assert"},
	{"console", "clear"},
	{"console", "count"},
	{"console", "countReset"},
	{"console", "debug"},
	{"console", "dir"},
	{"console", "dirxml"},
	{"console", "error"},
	{"console", "group"},
	{"console", "groupCollapsed"},
	{"console", "groupEnd"},
	{"console", "info"},
	{"console", "log"},
	{"console", "table"},
	{"console", "time"},
	{"console", "timeEnd"},
	{"console", "timeLog"},
	{"console", "trace"},
	{"console", "warn"},

	// CSSOM APIs
	{"CSSAnimation"},
	{"CSSFontFaceRule"},
	{"CSSImportRule"},
	{"CSSKeyframeRule"},
	{"CSSKeyframesRule"},
	{"CSSMediaRule"},
	{"CSSNamespaceRule"},
	{"CSSPageRule"},
	{"CSSRule"},
	{"CSSRuleList"},
	{"CSSStyleDeclaration"},
	{"CSSStyleRule"},
	{"CSSStyleSheet"},
	{"CSSSupportsRule"},
	{"CSSTransition"},

	// SVG DOM
	{"SVGAElement"},
	{"SVGAngle"},
	{"SVGAnimateElement"},
	{"SVGAnimateMotionElement"},
	{"SVGAnimateTransformElement"},
	{"SVGAnimatedAngle"},
	{"SVGAnimatedBoolean"},
	{"SVGAnimatedEnumeration"},
	{"SVGAnimatedInteger"},
	{"SVGAnimatedLength"},
	{"SVGAnimatedLengthList"},
	{"SVGAnimatedNumber"},
	{"SVGAnimatedNumberList"},
	{"SVGAnimatedPreserveAspectRatio"},
	{"SVGAnimatedRect"},
	{"SVGAnimatedString"},
	{"SVGAnimatedTransformList"},
	{"SVGAnimationElement"},
	{"SVGCircleElement"},
	{"SVGClipPathElement"},
	{"SVGComponentTransferFunctionElement"},
	{"SVGDefsElement"},
	{"SVGDescElement"},
	{"SVGElement"},
	{"SVGEllipseElement"},
	{"SVGFEBlendElement"},
	{"SVGFEColorMatrixElement"},
	{"SVGFEComponentTransferElement"},
	{"SVGFECompositeElement"},
	{"SVGFEConvolveMatrixElement"},
	{"SVGFEDiffuseLightingElement"},
	{"SVGFEDisplacementMapElement"},
	{"SVGFEDistantLightElement"},
	{"SVGFEDropShadowElement"},
	{"SVGFEFloodElement"},
	{"SVGFEFuncAElement"},
	{"SVGFEFuncBElement"},
	{"SVGFEFuncGElement"},
	{"SVGFEFuncRElement"},
	{"SVGFEGaussianBlurElement"},
	{"SVGFEImageElement"},
	{"SVGFEMergeElement"},
	{"SVGFEMergeNodeElement"},
	{"SVGFEMorphologyElement"},
	{"SVGFEOffsetElement"},
	{"SVGFEPointLightElement"},
	{"SVGFESpecularLightingElement"},
	{"SVGFESpotLightElement"},
	{"SVGFETileElement"},
	{"SVGFETurbulenceElement"},
	{"SVGFilterElement"},
	{"SVGForeignObjectElement"},
	{"SVGGElement"},
	{"SVGGeometryElement"},
	{"SVGGradientElement"},
	{"SVGGraphicsElement"},
	{"SVGImageElement"},
	{"SVGLength"},
	{"SVGLengthList"},
	{"SVGLineElement"},
	{"SVGLinearGradientElement"},
	{"SVGMPathElement"},
	{"SVGMarkerElement"},
	{"SVGMaskElement"},
	{"SVGMatrix"},
	{"SVGMetadataElement"},
	{"SVGNumber"},
	{"SVGNumberList"},
	{"SVGPathElement"},
	{"SVGPatternElement"},
	{"SVGPoint"},
	{"SVGPointList"},
	{"SVGPolygonElement"},
	{"SVGPolylineElement"},
	{"SVGPreserveAspectRatio"},
	{"SVGRadialGradientElement"},
	{"SVGRect"},
	{"SVGRectElement"},
	{"SVGSVGElement"},
	{"SVGScriptElement"},
	{"SVGSetElement"},
	{"SVGStopElement"},
	{"SVGStringList"},
	{"SVGStyleElement"},
	{"SVGSwitchElement"},
	{"SVGSymbolElement"},
	{"SVGTSpanElement"},
	{"SVGTextContentElement"},
	{"SVGTextElement"},
	{"SVGTextPathElement"},
	{"SVGTextPositioningElement"},
	{"SVGTitleElement"},
	{"SVGTransform"},
	{"SVGTransformList"},
	{"SVGUnitTypes"},
	{"SVGUseElement"},
	{"SVGViewElement"},

	// Other browser APIs
	//
	// This list contains all globals present in modern versions of Chrome, Safari,
	// and Firefox except for the following properties, since they have a side effect
	// of triggering layout (https://gist.github.com/paulirish/5d52fb081b3570c81e3a):
	//
	//   - scrollX
	//   - scrollY
	//   - innerWidth
	//   - innerHeight
	//   - pageXOffset
	//   - pageYOffset
	//
	// The following globals have also been removed since they sometimes throw an
	// exception when accessed, which is a side effect (for more information see
	// https://stackoverflow.com/a/33047477):
	//
	//   - localStorage
	//   - sessionStorage
	//
	{"AnalyserNode"},
	{"Animation"},
	{"AnimationEffect"},
	{"AnimationEvent"},
	{"AnimationPlaybackEvent"},
	{"AnimationTimeline"},
	{"Attr"},
	{"Audio"},
	{"AudioBuffer"},
	{"AudioBufferSourceNode"},
	{"AudioDestinationNode"},
	{"AudioListener"},
	{"AudioNode"},
	{"AudioParam"},
	{"AudioProcessingEvent"},
	{"AudioScheduledSourceNode"},
	{"BarProp"},
	{"BeforeUnloadEvent"},
	{"BiquadFilterNode"},
	{"Blob"},
	{"BlobEvent"},
	{"ByteLengthQueuingStrategy"},
	{"CDATASection"},
	{"CSS"},
	{"CanvasGradient"},
	{"CanvasPattern"},
	{"CanvasRenderingContext2D"},
	{"ChannelMergerNode"},
	{"ChannelSplitterNode"},
	{"CharacterData"},
	{"ClipboardEvent"},
	{"CloseEvent"},
	{"Comment"},
	{"CompositionEvent"},
	{"ConvolverNode"},
	{"CountQueuingStrategy"},
	{"Crypto"},
	{"CustomElementRegistry"},
	{"CustomEvent"},
	{"DOMException"},
	{"DOMImplementation"},
	{"DOMMatrix"},
	{"DOMMatrixReadOnly"},
	{"DOMParser"},
	{"DOMPoint"},
	{"DOMPointReadOnly"},
	{"DOMQuad"},
	{"DOMRect"},
	{"DOMRectList"},
	{"DOMRectReadOnly"},
	{"DOMStringList"},
	{"DOMStringMap"},
	{"DOMTokenList"},
	{"DataTransfer"},
	{"DataTransferItem"},
	{"DataTransferItemList"},
	{"DelayNode"},
	{"Document"},
	{"DocumentFragment"},
	{"DocumentTimeline"},
	{"DocumentType"},
	{"DragEvent"},
	{"DynamicsCompressorNode"},
	{"Element"},
	{"ErrorEvent"},
	{"EventSource"},
	{"File"},
	{"FileList"},
	{"FileReader"},
	{"FocusEvent"},
	{"FontFace"},
	{"FormData"},
	{"GainNode"},
	{"Gamepad"},
	{"GamepadButton"},
	{"GamepadEvent"},
	{"Geolocation"},
	{"GeolocationPositionError"},
	{"HTMLAllCollection"},
	{"HTMLAnchorElement"},
	{"HTMLAreaElement"},
	{"HTMLAudioElement"},
	{"HTMLBRElement"},
	{"HTMLBaseElement"},
	{"HTMLBodyElement"},
	{"HTMLButtonElement"},
	{"HTMLCanvasElement"},
	{"HTMLCollection"},
	{"HTMLDListElement"},
	{"HTMLDataElement"},
	{"HTMLDataListElement"},
	{"HTMLDetailsElement"},
	{"HTMLDirectoryElement"},
	{"HTMLDivElement"},
	{"HTMLDocument"},
	{"HTMLElement"},
	{"HTMLEmbedElement"},
	{"HTMLFieldSetElement"},
	{"HTMLFontElement"},
	{"HTMLFormControlsCollection"},
	{"HTMLFormElement"},
	{"HTMLFrameElement"},
	{"HTMLFrameSetElement"},
	{"HTMLHRElement"},
	{"HTMLHeadElement"},
	{"HTMLHeadingElement"},
	{"HTMLHtmlElement"},
	{"HTMLIFrameElement"},
	{"HTMLImageElement"},
	{"HTMLInputElement"},
	{"HTMLLIElement"},
	{"HTMLLabelElement"},
	{"HTMLLegendElement"},
	{"HTMLLinkElement"},
	{"HTMLMapElement"},
	{"HTMLMarqueeElement"},
	{"HTMLMediaElement"},
	{"HTMLMenuElement"},
	{"HTMLMetaElement"},
	{"HTMLMeterElement"},
	{"HTMLModElement"},
	{"HTMLOListElement"},
	{"HTMLObjectElement"},
	{"HTMLOptGroupElement"},
	{"HTMLOptionElement"},
	{"HTMLOptionsCollection"},
	{"HTMLOutputElement"},
	{"HTMLParagraphElement"},
	{"HTMLParamElement"},
	{"HTMLPictureElement"},
	{"HTMLPreElement"},
	{"HTMLProgressElement"},
	{"HTMLQuoteElement"},
	{"HTMLScriptElement"},
	{"HTMLSelectElement"},
	{"HTMLSlotElement"},
	{"HTMLSourceElement"},
	{"HTMLSpanElement"},
	{"HTMLStyleElement"},
	{"HTMLTableCaptionElement"},
	{"HTMLTableCellElement"},
	{"HTMLTableColElement"},
	{"HTMLTableElement"},
	{"HTMLTableRowElement"},
	{"HTMLTableSectionElement"},
	{"HTMLTemplateElement"},
	{"HTMLTextAreaElement"},
	{"HTMLTimeElement"},
	{"HTMLTitleElement"},
	{"HTMLTrackElement"},
	{"HTMLUListElement"},
	{"HTMLUnknownElement"},
	{"HTMLVideoElement"},
	{"HashChangeEvent"},
	{"Headers"},
	{"History"},
	{"IDBCursor"},
	{"IDBCursorWithValue"},
	{"IDBDatabase"},
	{"IDBFactory"},
	{"IDBIndex"},
	{"IDBKeyRange"},
	{"IDBObjectStore"},
	{"IDBOpenDBRequest"},
	{"IDBRequest"},
	{"IDBTransaction"},
	{"IDBVersionChangeEvent"},
	{"Image"},
	{"ImageData"},
	{"InputEvent"},
	{"IntersectionObserver"},
	{"IntersectionObserverEntry"},
	{"KeyboardEvent"},
	{"KeyframeEffect"},
	{"Location"},
	{"MediaCapabilities"},
	{"MediaElementAudioSourceNode"},
	{"MediaEncryptedEvent"},
	{"MediaError"},
	{"MediaList"},
	{"MediaQueryList"},
	{"MediaQueryListEvent"},
	{"MediaRecorder"},
	{"MediaSource"},
	{"MediaStream"},
	{"MediaStreamAudioDestinationNode"},
	{"MediaStreamAudioSourceNode"},
	{"MediaStreamTrack"},
	{"MediaStreamTrackEvent"},
	{"MimeType"},
	{"MimeTypeArray"},
	{"MouseEvent"},
	{"MutationEvent"},
	{"MutationObserver"},
	{"MutationRecord"},
	{"NamedNodeMap"},
	{"Navigator"},
	{"Node"},
	{"NodeFilter"},
	{"NodeIterator"},
	{"NodeList"},
	{"Notification"},
	{"OfflineAudioCompletionEvent"},
	{"Option"},
	{"OscillatorNode"},
	{"PageTransitionEvent"},
	{"Path2D"},
	{"Performance"},
	{"PerformanceEntry"},
	{"PerformanceMark"},
	{"PerformanceMeasure"},
	{"PerformanceNavigation"},
	{"PerformanceObserver"},
	{"PerformanceObserverEntryList"},
	{"PerformanceResourceTiming"},
	{"PerformanceTiming"},
	{"PeriodicWave"},
	{"Plugin"},
	{"PluginArray"},
	{"PointerEvent"},
	{"PopStateEvent"},
	{"ProcessingInstruction"},
	{"ProgressEvent"},
	{"PromiseRejectionEvent"},
	{"RTCCertificate"},
	{"RTCDTMFSender"},
	{"RTCDTMFToneChangeEvent"},
	{"RTCDataChannel"},
	{"RTCDataChannelEvent"},
	{"RTCIceCandidate"},
	{"RTCPeerConnection"},
	{"RTCPeerConnectionIceEvent"},
	{"RTCRtpReceiver"},
	{"RTCRtpSender"},
	{"RTCRtpTransceiver"},
	{"RTCSessionDescription"},
	{"RTCStatsReport"},
	{"RTCTrackEvent"},
	{"RadioNodeList"},
	{"Range"},
	{"ReadableStream"},
	{"Request"},
	{"ResizeObserver"},
	{"ResizeObserverEntry"},
	{"Response"},
	{"Screen"},
	{"ScriptProcessorNode"},
	{"SecurityPolicyViolationEvent"},
	{"Selection"},
	{"ShadowRoot"},
	{"SourceBuffer"},
	{"SourceBufferList"},
	{"SpeechSynthesisEvent"},
	{"SpeechSynthesisUtterance"},
	{"StaticRange"},
	{"Storage"},
	{"StorageEvent"},
	{"StyleSheet"},
	{"StyleSheetList"},
	{"Text"},
	{"TextMetrics"},
	{"TextTrack"},
	{"TextTrackCue"},
	{"TextTrackCueList"},
	{"TextTrackList"},
	{"TimeRanges"},
	{"TrackEvent"},
	{"TransitionEvent"},
	{"TreeWalker"},
	{"UIEvent"},
	{"VTTCue"},
	{"ValidityState"},
	{"VisualViewport"},
	{"WaveShaperNode"},
	{"WebGLActiveInfo"},
	{"WebGLBuffer"},
	{"WebGLContextEvent"},
	{"WebGLFramebuffer"},
	{"WebGLProgram"},
	{"WebGLQuery"},
	{"WebGLRenderbuffer"},
	{"WebGLRenderingContext"},
	{"WebGLSampler"},
	{"WebGLShader"},
	{"WebGLShaderPrecisionFormat"},
	{"WebGLSync"},
	{"WebGLTexture"},
	{"WebGLUniformLocation"},
	{"WebKitCSSMatrix"},
	{"WebSocket"},
	{"WheelEvent"},
	{"Window"},
	{"Worker"},
	{"XMLDocument"},
	{"XMLHttpRequest"},
	{"XMLHttpRequestEventTarget"},
	{"XMLHttpRequestUpload"},
	{"XMLSerializer"},
	{"XPathEvaluator"},
	{"XPathExpression"},
	{"XPathResult"},
	{"XSLTProcessor"},
	{"alert"},
	{"atob"},
	{"blur"},
	{"btoa"},
	{"cancelAnimationFrame"},
	{"captureEvents"},
	{"close"},
	{"closed"},
	{"confirm"},
	{"customElements"},
	{"devicePixelRatio"},
	{"document"},
	{"event"},
	{"fetch"},
	{"find"},
	{"focus"},
	{"frameElement"},
	{"frames"},
	{"getComputedStyle"},
	{"getSelection"},
	{"history"},
	{"indexedDB"},
	{"isSecureContext"},
	{"length"},
	{"location"},
	{"locationbar"},
	{"matchMedia"},
	{"menubar"},
	{"moveBy"},
	{"moveTo"},
	{"name"},
	{"navigator"},
	{"onabort"},
	{"onafterprint"},
	{"onanimationend"},
	{"onanimationiteration"},
	{"onanimationstart"},
	{"onbeforeprint"},
	{"onbeforeunload"},
	{"onblur"},
	{"oncanplay"},
	{"oncanplaythrough"},
	{"onchange"},
	{"onclick"},
	{"oncontextmenu"},
	{"oncuechange"},
	{"ondblclick"},
	{"ondrag"},
	{"ondragend"},
	{"ondragenter"},
	{"ondragleave"},
	{"ondragover"},
	{"ondragstart"},
	{"ondrop"},
	{"ondurationchange"},
	{"onemptied"},
	{"onended"},
	{"onerror"},
	{"onfocus"},
	{"ongotpointercapture"},
	{"onhashchange"},
	{"oninput"},
	{"oninvalid"},
	{"onkeydown"},
	{"onkeypress"},
	{"onkeyup"},
	{"onlanguagechange"},
	{"onload"},
	{"onloadeddata"},
	{"onloadedmetadata"},
	{"onloadstart"},
	{"onlostpointercapture"},
	{"onmessage"},
	{"onmousedown"},
	{"onmouseenter"},
	{"onmouseleave"},
	{"onmousemove"},
	{"onmouseout"},
	{"onmouseover"},
	{"onmouseup"},
	{"onoffline"},
	{"ononline"},
	{"onpagehide"},
	{"onpageshow"},
	{"onpause"},
	{"onplay"},
	{"onplaying"},
	{"onpointercancel"},
	{"onpointerdown"},
	{"onpointerenter"},
	{"onpointerleave"},
	{"onpointermove"},
	{"onpointerout"},
	{"onpointerover"},
	{"onpointerup"},
	{"onpopstate"},
	{"onprogress"},
	{"onratechange"},
	{"onrejectionhandled"},
	{"onreset"},
	{"onresize"},
	{"onscroll"},
	{"onseeked"},
	{"onseeking"},
	{"onselect"},
	{"onstalled"},
	{"onstorage"},
	{"onsubmit"},
	{"onsuspend"},
	{"ontimeupdate"},
	{"ontoggle"},
	{"ontransitioncancel"},
	{"ontransitionend"},
	{"ontransitionrun"},
	{"ontransitionstart"},
	{"onunhandledrejection"},
	{"onunload"},
	{"onvolumechange"},
	{"onwaiting"},
	{"onwebkitanimationend"},
	{"onwebkitanimationiteration"},
	{"onwebkitanimationstart"},
	{"onwebkittransitionend"},
	{"onwheel"},
	{"open"},
	{"opener"},
	{"origin"},
	{"outerHeight"},
	{"outerWidth"},
	{"parent"},
	{"performance"},
	{"personalbar"},
	{"postMessage"},
	{"print"},
	{"prompt"},
	{"releaseEvents"},
	{"requestAnimationFrame"},
	{"resizeBy"},
	{"resizeTo"},
	{"screen"},
	{"screenLeft"},
	{"screenTop"},
	{"screenX"},
	{"screenY"},
	{"scroll"},
	{"scrollBy"},
	{"scrollTo"},
	{"scrollbars"},
	{"self"},
	{"speechSynthesis"},
	{"status"},
	{"statusbar"},
	{"stop"},
	{"toolbar"},
	{"top"},
	{"webkitURL"},
	{"window"},
}

type DefineData struct {

	// True if accessing this value is known to not have any side effects. For
	// example, a bare reference to "Object.create" can be removed because it
	// does not have any observable side effects.
	CanBeRemovedIfUnused bool

	// True if a call to this value is known to not have any side effects. For
	// example, a bare call to "Object()" can be removed because it does not
	// have any observable side effects.
	CallCanBeUnwrappedIfUnused bool

	// If true, the user has indicated that every direct calls to a property on
	// this object and all of that call's arguments are to be removed from the
	// output, even when the arguments have side effects. This is used to
	// implement the "--drop:console" flag.
	MethodCallsMustBeReplacedWithUndefined bool
}

func mergeDefineData(old DefineData, new DefineData) DefineData {
	if old.CanBeRemovedIfUnused {
		new.CanBeRemovedIfUnused = true
	}
	if old.CallCanBeUnwrappedIfUnused {
		new.CallCanBeUnwrappedIfUnused = true
	}
	return new
}

type DotDefine struct {
	Data  DefineData
	Parts []string
}

func arePartsEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
