// Enables scroll-to-zoom / drag-to-pan on rendered Mermaid diagrams using
// svg-pan-zoom (vendored in this same directory). Mermaid renders its <svg>
// asynchronously after the page loads, and Material's instant navigation
// swaps page content without a full reload, so we:
//   1. re-run on every page load via document$ (Material's navigation hook)
//   2. watch each `.mermaid` container with a MutationObserver until its
//      <svg> child actually appears before initializing pan/zoom on it
document$.subscribe(function () {
  document.querySelectorAll(".mermaid").forEach(function (container) {
    if (container.dataset.panZoomInit) {
      return;
    }

    var attach = function (svg) {
      if (!svg || svg.dataset.panZoomInit) {
        return;
      }
      svg.dataset.panZoomInit = "true";
      container.dataset.panZoomInit = "true";
      svg.style.maxWidth = "100%";
      svg.style.height = "auto";
      window.svgPanZoom(svg, {
        zoomEnabled: true,
        controlIconsEnabled: true,
        fit: true,
        center: true,
        minZoom: 0.5,
        maxZoom: 10,
      });
    };

    var existing = container.querySelector("svg");
    if (existing) {
      attach(existing);
      return;
    }

    var observer = new MutationObserver(function () {
      var svg = container.querySelector("svg");
      if (svg) {
        attach(svg);
        observer.disconnect();
      }
    });
    observer.observe(container, { childList: true, subtree: true });
  });
});
