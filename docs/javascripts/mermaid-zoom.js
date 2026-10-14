// Enables scroll-to-zoom / drag-to-pan on rendered Mermaid diagrams using
// svg-pan-zoom (vendored in this same directory).
//
// Material renders each diagram's <svg> into a *closed* shadow root
// (`element.attachShadow({mode: "closed"})`) for style isolation, which
// makes it permanently inaccessible to any outside script, including this
// one — `container.shadowRoot` is always null and `container.querySelector`
// can never see inside it. The standard workaround (used widely for
// mkdocs-material + mermaid zoom integrations) is to force every shadow
// root open *before* Material creates it, by overriding
// `Element.prototype.attachShadow` once, globally, as early as possible.
// This must run before Material's lazy, intersection-observer-triggered
// render call, which it reliably does since this script executes
// synchronously right after bundle.js loads.
(function () {
  var nativeAttachShadow = Element.prototype.attachShadow;
  Element.prototype.attachShadow = function (options) {
    return nativeAttachShadow.call(this, { ...options, mode: "open" });
  };
})();

// Mermaid renders its <svg> asynchronously after the page loads, and
// Material replaces the original `.mermaid` element outright (rather than
// mutating it in place) once rendering finishes, so instead of tracking one
// specific container across that replacement, we just rescan the whole page
// for not-yet-initialized `.mermaid` elements:
//   1. on every page load via document$ (Material's navigation hook)
//   2. on every subsequent DOM mutation, via a single page-wide observer,
//      until each diagram's <svg> (inside its now-open shadow root) appears
function attach(svg, container) {
  if (!svg || svg.dataset.panZoomInit) {
    return;
  }
  svg.dataset.panZoomInit = "true";
  container.dataset.panZoomInit = "true";
  // Fill the whole (now explicitly tall, see extra.css) container instead of
  // "height: auto", which would just shrink the svg back down to the
  // diagram's natural width-driven aspect ratio. svg-pan-zoom's `fit` then
  // scales the diagram up to use all of that real estate.
  svg.style.width = "100%";
  svg.style.height = "100%";
  var instance = window.svgPanZoom(svg, {
    zoomEnabled: true,
    controlIconsEnabled: true,
    fit: true,
    center: true,
    minZoom: 0.5,
    maxZoom: 10,
  });
  // The svg's layout box isn't always final the instant svg-pan-zoom reads
  // it (web fonts / shadow DOM style application can still be settling),
  // which can leave the initial fit/center slightly off. Re-run both once
  // more a tick later, once layout has definitely stabilized.
  window.requestAnimationFrame(function () {
    instance.resize();
    instance.fit();
    instance.center();
  });
}

function scanForDiagrams() {
  document.querySelectorAll(".mermaid").forEach(function (container) {
    if (container.dataset.panZoomInit) {
      return;
    }
    var svg = (container.shadowRoot || container).querySelector("svg");
    if (svg) {
      attach(svg, container);
    }
  });
}

document$.subscribe(function () {
  scanForDiagrams();

  if (window.__mermaidZoomObserver) {
    return;
  }
  window.__mermaidZoomObserver = new MutationObserver(scanForDiagrams);
  window.__mermaidZoomObserver.observe(document.body, {
    childList: true,
    subtree: true,
  });
});
