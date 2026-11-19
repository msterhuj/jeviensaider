// Small progressive enhancements; every feature also works without JS.
(function () {
  "use strict";

  // Stand descriptions in the overview are cut after two lines: mark the cut
  // ones, so that their "Voir plus" link shows.
  function markClamped(root) {
    root.querySelectorAll(".stand-desc:not(.checked)").forEach(function (d) {
      d.classList.add("checked");
      if (d.scrollHeight > d.clientHeight + 1) d.classList.add("clamped");
    });
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () { markClamped(document); });
  } else {
    markClamped(document);
  }
  document.addEventListener("htmx:afterSettle", function (e) { markClamped(e.detail.elt || document); });

  // Sign-up form loaded by htmx into #modal: open it as a modal dialog.
  document.addEventListener("htmx:afterSwap", function (e) {
    if (e.detail.target && e.detail.target.id === "modal") {
      var d = e.detail.target.querySelector("dialog");
      if (d && !d.open) {
        d.addEventListener("close", function () { d.remove(); });
        d.showModal();
        var first = d.querySelector("input:not([type=hidden]):not([tabindex='-1'])");
        if (first) first.focus();
      }
    }
  });

  // copyText copies a field's value and confirms on the button.
  function copyText(input, button) {
    var done = function () {
      var old = button.innerHTML;
      button.textContent = "Copié ✓";
      setTimeout(function () { button.innerHTML = old; }, 1500);
    };
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(input.value).then(done);
    } else {
      input.select();
      try { document.execCommand("copy"); done(); } catch (_) {}
    }
  }

  document.addEventListener("click", function (e) {
    var t = e.target;
    if (!(t instanceof Element)) return;

    // Close button inside a dialog.
    var close = t.closest("[data-close]");
    if (close) {
      var d = close.closest("dialog");
      if (d) d.close();
      return;
    }
    // Click on the backdrop closes the dialog.
    if (t.tagName === "DIALOG" && t.open) {
      var r = t.getBoundingClientRect();
      if (e.clientX < r.left || e.clientX > r.right || e.clientY < r.top || e.clientY > r.bottom) t.close();
      return;
    }

    // Copy-to-clipboard buttons: data-copy="#input-id".
    var copy = t.closest("[data-copy]");
    if (copy) {
      var input = document.querySelector(copy.getAttribute("data-copy"));
      if (input) copyText(input, copy);
      return;
    }

    // Share a prepared message (phone share sheet), else copy it:
    // data-share="#textarea-id".
    var share = t.closest("[data-share]");
    if (share) {
      var src = document.querySelector(share.getAttribute("data-share"));
      if (!src) return;
      if (navigator.share) {
        navigator.share({ text: src.value }).catch(function () {});
      } else {
        copyText(src, share);
      }
      return;
    }

    if (t.closest("[data-back]")) { history.back(); return; }
  });

  // Confirmation for destructive forms: <form data-confirm="Sûr ?">.
  document.addEventListener("submit", function (e) {
    var f = e.target;
    var msg = f.getAttribute && f.getAttribute("data-confirm");
    if (msg && !window.confirm(msg)) e.preventDefault();
  }, true);

  // Select-on-focus for read-only link fields.
  document.addEventListener("focusin", function (e) {
    var t = e.target;
    if (t.tagName === "INPUT" && t.readOnly) t.select();
  });
})();
