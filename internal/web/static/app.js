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

  // Slot generator: preview the slots the form will create, the same way
  // the server cuts them (internal/slotgen): consecutive slots of the chosen
  // length, the last one shorter if the range is not a multiple of it.
  var MAX_SLOTS = 96;
  function hm(m) { var h = Math.floor(m / 60), r = m % 60; return h + "h" + (r ? (r < 10 ? "0" : "") + r : ""); }
  function minutes(v) { var p = /^(\d{1,2}):(\d{2})/.exec(v || ""); return p ? +p[1] * 60 + +p[2] : null; }
  function previewSlots(form) {
    var box = form.querySelector(".slot-preview");
    var submit = form.querySelector("[data-preview-submit]");
    if (!box) return;
    var from = minutes(form.elements.from.value), to = minutes(form.elements.to.value);
    var stepInput = form.querySelector("input[name=step]:checked");
    var step = stepInput ? +stepInput.value : 60;
    var min = parseInt(form.elements.min.value, 10);
    box.hidden = false;
    box.textContent = "";
    box.classList.remove("error");
    var fail = function (msg) { box.classList.add("error"); box.textContent = msg; if (submit) submit.textContent = "Créer les créneaux"; };
    if (from === null || to === null) { box.hidden = true; return; }
    if (to <= from) { fail("L'heure de fin doit être après l'heure de début."); return; }
    if (!step || step > to - from) step = to - from;
    var slots = [];
    for (var t = from; t < to; t += step) slots.push([t, Math.min(t + step, to)]);
    if (slots.length > MAX_SLOTS) { fail("Trop de créneaux (" + MAX_SLOTS + " au maximum) : choisissez une durée plus longue."); return; }
    var n = slots.length;
    var title = document.createElement("strong");
    title.textContent = "Cela crée " + n + " créneau" + (n > 1 ? "x" : "") +
      (min > 0 ? ", " + n * min + " place" + (n * min > 1 ? "s" : "") + " à remplir" : "");
    var list = document.createElement("ul");
    slots.forEach(function (s) {
      var li = document.createElement("li");
      li.textContent = hm(s[0]) + " – " + hm(s[1]);
      list.appendChild(li);
    });
    box.appendChild(title);
    box.appendChild(list);
    if (submit) submit.textContent = n > 1 ? "Créer les " + n + " créneaux" : "Créer le créneau";
  }
  function initPreviews(root) {
    root.querySelectorAll("form[data-slot-preview]").forEach(previewSlots);
  }
  document.addEventListener("input", function (e) {
    var f = e.target.form;
    if (f && f.hasAttribute("data-slot-preview")) previewSlots(f);
  });
  document.addEventListener("change", function (e) {
    var f = e.target.form;
    if (f && f.hasAttribute("data-slot-preview")) previewSlots(f);
  });
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () { initPreviews(document); });
  } else {
    initPreviews(document);
  }

  // Select-on-focus for read-only link fields.
  document.addEventListener("focusin", function (e) {
    var t = e.target;
    if (t.tagName === "INPUT" && t.readOnly) t.select();
  });
})();
