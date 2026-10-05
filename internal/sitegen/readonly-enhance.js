// Runtime enhancement for the generated static site.
//
// The exported HTML matches the app's reading DOM, but heavy renderers
// (highlight.js, KaTeX, Mermaid) are produced in the browser. Only the
// libraries a page needs are loaded, and asset URLs honour --base subpaths.
(function () {
  "use strict";

  // Syntax highlighting: match web/src/editor/lowlight.ts (highlight.js
  // "common" set). We call hljs.highlight directly so no root `hljs` class is
  // added, matching lowlight's token output.
  var codes = document.querySelectorAll('pre.code-block > code[class*="language-"]');
  if (codes.length && window.hljs) {
    Array.prototype.forEach.call(codes, function (code) {
      var match = /language-([\w-]+)/.exec(code.className || "");
      var lang = match && match[1];
      if (!lang || !window.hljs.getLanguage(lang)) return;
      try {
        code.innerHTML = window.hljs.highlight(code.textContent, { language: lang }).value;
      } catch (e) {
        /* leave the plain source in place */
      }
    });
  }

  // Math: mirror web/src/markdown/math.ts renderMath options.
  var maths = document.querySelectorAll(".math-inline[data-tex], .math-block[data-tex]");
  if (maths.length && window.katex) {
    Array.prototype.forEach.call(maths, function (el) {
      var display = el.classList.contains("math-block");
      try {
        el.innerHTML = window.katex.renderToString(el.getAttribute("data-tex") || "", {
          displayMode: display,
          throwOnError: false,
          output: "html",
        });
      } catch (e) {
        el.classList.add("math-error");
      }
    });
  }

  // Mermaid: mirror web/src/markdown/diagrams.ts initialization.
  var diagrams = document.querySelectorAll(".mermaid[data-source]:not([data-processed])");
  if (diagrams.length && window.mermaid) {
    window.mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: document.documentElement.dataset.theme === "dark" ? "dark" : "default",
      fontFamily: "var(--font-sans)",
    });
    var seq = 0;
    Array.prototype.forEach.call(diagrams, function (el) {
      el.dataset.processed = "1";
      var source = el.getAttribute("data-source") || "";
      var id = "mmd-" + Date.now() + "-" + seq++;
      Promise.resolve(window.mermaid.render(id, source))
        .then(function (result) {
          el.innerHTML = result.svg;
          el.classList.add("mermaid-rendered");
        })
        .catch(function (err) {
          el.classList.add("mermaid-error");
          el.textContent = source + "\n\n" + (err && err.message ? err.message : String(err));
        });
    });
  }
  // Footnotes: the app keeps them as inert sup/div pairs, so make the reference
  // scroll to its definition (no DOM shape change).
  var refs = document.querySelectorAll(".fn-ref[data-id]");
  if (refs.length) {
    var defs = document.querySelectorAll(".fn-def[data-id]");
    Array.prototype.forEach.call(refs, function (ref) {
      ref.style.cursor = "pointer";
      ref.addEventListener("click", function () {
        var id = ref.getAttribute("data-id");
        for (var i = 0; i < defs.length; i++) {
          if (defs[i].getAttribute("data-id") === id) {
            defs[i].scrollIntoView({ behavior: "smooth", block: "center" });
            return;
          }
        }
      });
    });
  }
})();
