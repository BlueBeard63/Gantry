// Gantry test report self-contained viewer.
// Reads the inlined RunReport JSON and renders the Run Overview and the
// per-test Detail (Screencast / Screenshots / Trace / Logs). No network,
// no framework everything the report needs is in this one file.

(function () {
  "use strict";

  var DATA = {};
  try {
    DATA = JSON.parse(document.getElementById("gantry-report").textContent || "{}");
  } catch (e) { DATA = {}; }
  DATA.files = DATA.files || [];
  DATA.counts = DATA.counts || {};

  var app = document.getElementById("app");

  // ---- Lucide icons (inline SVG, stroked, currentColor) ----
  var ICONS = {
    search: '<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>',
    chevron: '<path d="m6 9 6 6 6-6"/>',
    left: '<path d="m15 18-6-6 6-6"/>',
    right: '<path d="m9 18 6-6-6-6"/>',
    check: '<path d="M20 6 9 17l-5-5"/>',
    x: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
    retry: '<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/>',
    minus: '<path d="M5 12h14"/>',
    play: '<polygon points="6 3 20 12 6 21 6 3"/>',
    pause: '<rect x="14" y="4" width="4" height="16" rx="1"/><rect x="6" y="4" width="4" height="16" rx="1"/>',
    download: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/>',
    maximize: '<path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/>',
    volume: '<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/><path d="M15.54 8.46a5 5 0 0 1 0 7.07"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14"/>',
    mute: '<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/><line x1="22" x2="16" y1="9" y2="15"/><line x1="16" x2="22" y1="9" y2="15"/>',
    image: '<rect width="18" height="18" x="3" y="3" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/>',
    film: '<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M7 3v18"/><path d="M3 7.5h4"/><path d="M3 12h18"/><path d="M3 16.5h4"/><path d="M17 3v18"/><path d="M17 7.5h4"/><path d="M17 16.5h4"/>',
    file: '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 9H8"/><path d="M16 13H8"/><path d="M16 17H8"/>',
    activity: '<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>',
    alert: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/>',
    info: '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>',
    back: '<path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>',
    copy: '<rect width="14" height="14" x="8" y="8" rx="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>',
    external: '<path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>',
  };
  function icon(name, size) {
    var s = document.createElement("span");
    s.className = "ic";
    s.innerHTML = '<svg viewBox="0 0 24 24" width="' + (size || 16) + '" height="' + (size || 16) +
      '" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' + (ICONS[name] || "") + "</svg>";
    return s;
  }
  var STATUS_ICON = { pass: "check", fail: "x", flaky: "retry", skip: "minus" };

  // ---- tiny DOM helper ----
  function el(tag, attrs, children) {
    var n = document.createElement(tag);
    if (attrs) {
      for (var k in attrs) {
        if (k === "class") n.className = attrs[k];
        else if (k === "text") n.textContent = attrs[k];
        else if (k === "html") n.innerHTML = attrs[k];
        else if (k.slice(0, 2) === "on") n.addEventListener(k.slice(2), attrs[k]);
        else if (attrs[k] != null) n.setAttribute(k, attrs[k]);
      }
    }
    (children || []).forEach(function (c) {
      if (c == null) return;
      n.appendChild(typeof c === "string" ? document.createTextNode(c) : c);
    });
    return n;
  }
  function clear(n) { while (n.firstChild) n.removeChild(n.firstChild); }

  // ---- formatting ----
  function fmtDur(s) {
    if (s == null) return "–";
    if (s < 0.0005) return "0s";
    return s.toFixed(s < 100 ? 1 : 0) + "s";
  }
  function fmtBytes(b) {
    if (!b) return "–";
    if (b < 1024) return b + " B";
    if (b < 1024 * 1024) return (b / 1024).toFixed(0) + " KB";
    return (b / 1024 / 1024).toFixed(1) + " MB";
  }
  function allTests() {
    var out = [];
    DATA.files.forEach(function (f) {
      f.tests.forEach(function (t) { t._file = f.path; out.push(t); });
    });
    return out;
  }
  function findTest(name) {
    return allTests().filter(function (t) { return t.name === name; })[0];
  }
  function finalAttempt(t) { return (t.attempts || [])[(t.attempts || []).length - 1] || {}; }

  // ---- routing ----
  function parseHash() {
    var h = location.hash.replace(/^#/, "");
    var q = {};
    h.split("&").forEach(function (p) {
      var i = p.indexOf("=");
      if (i >= 0) q[p.slice(0, i)] = decodeURIComponent(p.slice(i + 1));
    });
    return q;
  }
  function go(hash) { location.hash = hash; }

  function render() {
    var q = parseHash();
    clear(app);
    app.appendChild(header());
    var wrap = el("div", { class: "wrap" });
    app.appendChild(wrap);
    if (q.test) renderDetail(wrap, q.test, q.tab || "screencast", parseInt(q.att || "0", 10));
    else renderOverview(wrap);
    window.scrollTo(0, 0);
  }
  window.addEventListener("hashchange", render);

  // ---- header ----
  function header() {
    var startedNice = "";
    if (DATA.startedAt) {
      var d = new Date(DATA.startedAt);
      if (!isNaN(d)) startedNice = d.toLocaleString();
    }
    return el("div", { class: "hdr" }, [
      el("div", { class: "brand" }, [
        el("div", { class: "logo" }),
        el("span", {}, ["Gantry ", el("span", { class: "sub", text: "test report" })]),
      ]),
      el("div", { class: "meta" }, [
        el("span", { class: "cmd", text: DATA.command || "gantry test" }),
        startedNice ? el("span", { class: "dot", text: "·" }) : null,
        startedNice ? el("span", { text: startedNice }) : null,
        el("span", { class: "dot", text: "·" }),
        el("span", { text: fmtDur(DATA.duration) }),
      ]),
    ]);
  }

  // ================= OVERVIEW =================
  var ovState = { q: "", status: "all" };

  function renderOverview(wrap) {
    var input = el("input", {
      placeholder: "Filter tests… (try s:failed or a:screencast)",
      value: ovState.q,
      oninput: function () { ovState.q = this.value; applyFilter(); },
    });
    wrap.appendChild(el("div", { class: "controls" }, [
      el("div", { class: "search" }, [icon("search"), input]),
      chips(),
    ]));
    var list = el("div", { id: "grouplist" });
    wrap.appendChild(list);
    drawGroups(list);
  }

  function chips() {
    var c = DATA.counts;
    var defs = [
      ["all", "All", c.total], ["passed", "Passed", c.passed], ["failed", "Failed", c.failed],
      ["flaky", "Flaky", c.flaky], ["skipped", "Skipped", c.skipped],
    ];
    var box = el("div", { class: "chips" });
    defs.forEach(function (d) {
      var active = ovState.status === d[0];
      box.appendChild(el("div", {
        class: "chip" + (active ? " active" : ""), "data-k": d[0],
        onclick: function () { ovState.status = d[0]; render(); },
      }, [d[1], el("span", { class: "n", text: String(d[2] || 0) })]));
    });
    return box;
  }

  function statusMatch(chip, st) {
    if (chip === "all") return true;
    if (chip === "passed") return st === "pass";
    if (chip === "failed") return st === "fail";
    if (chip === "flaky") return st === "flaky";
    if (chip === "skipped") return st === "skip";
    return true;
  }
  function testMatches(t) {
    if (!statusMatch(ovState.status, t.status)) return false;
    var terms = ovState.q.trim().toLowerCase().split(/\s+/).filter(Boolean);
    for (var i = 0; i < terms.length; i++) {
      var term = terms[i];
      if (term.indexOf("s:") === 0) {
        var want = term.slice(2);
        var st = t.status === "pass" ? "passed" : t.status === "fail" ? "failed" : t.status;
        if (st.indexOf(want) !== 0 && t.status.indexOf(want) !== 0) return false;
      } else if (term.indexOf("a:") === 0) {
        var av = term.slice(2);
        if (!artifactNames(t).some(function (n) { return n.indexOf(av) >= 0; })) return false;
      } else {
        if ((t.name.toLowerCase() + " " + (t._file || "").toLowerCase()).indexOf(term) < 0) return false;
      }
    }
    return true;
  }
  function artifactNames(t) {
    return (finalAttempt(t).artifacts || []).map(function (a) { return a.name.toLowerCase(); });
  }
  function applyFilter() { drawGroups(document.getElementById("grouplist")); }

  function drawGroups(list) {
    clear(list);
    var any = false;
    DATA.files.forEach(function (f) {
      var tests = f.tests.filter(testMatches);
      if (!tests.length) return;
      any = true;
      var g = el("div", { class: "group" });
      var rows = el("div", { class: "rows" });
      var head = el("div", { class: "ghead", onclick: function () { g.classList.toggle("collapsed"); } }, [
        el("span", { class: "caret" }, [icon("chevron")]),
        el("span", { class: "gpath", text: f.path }),
        el("span", { class: "gdur", text: fmtDur(f.duration) }),
      ]);
      g.appendChild(head);
      tests.forEach(function (t) { rows.appendChild(overviewRow(t)); });
      g.appendChild(rows);
      list.appendChild(g);
    });
    if (!any) list.appendChild(el("div", { class: "empty", text: "No tests match this filter." }));
  }

  function overviewRow(t) {
    var fin = finalAttempt(t);
    var top = el("div", { class: "top" }, [
      el("span", { class: "name", text: t.name }),
      planeBadge(t.plane),
    ]);
    if (t.status === "flaky") top.appendChild(el("span", { class: "badge retry", text: "passed on retry " + (t.attempts.length) }));
    var arts = fin.artifacts || [];
    arts.slice(0, 3).forEach(function (a) { top.appendChild(el("span", { class: "afile", text: a.name })); });
    if (arts.length > 3) top.appendChild(el("span", { class: "afile", text: "+" + (arts.length - 3) + " more" }));

    var durCls = "dur" + (t.status === "fail" ? " fail" : t.status === "flaky" ? " flaky" : "");
    return el("div", { class: "row", onclick: function () { go("test=" + encodeURIComponent(t.name)); } }, [
      el("span", { class: "st " + t.status }, [icon(STATUS_ICON[t.status] || "minus")]),
      el("div", { class: "body" }, [top, el("div", { class: "loc", text: loc(t) })]),
      el("span", { class: durCls, text: t.status === "skip" ? "–" : fmtDur(t.duration) }),
    ]);
  }
  function loc(t) { return t.file ? (baseName(t.file) + (t.line ? ":" + t.line : "")) : ""; }
  function baseName(p) { return (p || "").split("/").pop(); }
  function planeBadge(p) { return p ? el("span", { class: "badge " + p, text: p }) : el("span"); }

  // ================= DETAIL =================
  function renderDetail(wrap, name, tab, attN) {
    var t = findTest(name);
    if (!t) { wrap.appendChild(el("div", { class: "empty", text: "Unknown test: " + name })); return; }
    var attempts = t.attempts || [];
    var at = attN > 0 ? (attempts[attN - 1] || finalAttempt(t)) : finalAttempt(t);

    wrap.appendChild(el("div", { class: "detail-actions" }, [
      el("button", { class: "btn", onclick: function () { copy(at.dir || t.file, "Artifact path copied"); } }, [icon("copy"), "Copy artifact path"]),
      el("button", { class: "btn primary", onclick: function () { copy("gantry test -run '" + runPattern(t.name) + "'", "Re-run command copied"); } }, [icon("retry"), "Re-run this test"]),
    ]));
    wrap.appendChild(el("div", { class: "crumbs" }, [
      el("span", { class: "back", onclick: function () { go(""); } }, [icon("back"), "All tests"]),
      el("span", { class: "sep", text: "/" }),
      el("span", { class: "mono", text: t._file || t.file || "" }),
      el("span", { class: "sep", text: "/" }),
      el("span", { class: "cur", text: t.name }),
    ]));

    wrap.appendChild(el("div", { class: "dhead" }, [
      el("span", { class: "status-pill " + t.status }, [icon(STATUS_ICON[t.status] || "minus"), t.status.toUpperCase()]),
      el("span", { class: "dtitle", text: t.name }),
    ]));

    var meta = el("div", { class: "dmeta" }, [
      metaItem("plane", planeBadge(t.plane)),
      metaItem("duration", document.createTextNode(fmtDur(at.duration != null ? at.duration : t.duration))),
    ]);
    if (attempts.length > 1) meta.appendChild(metaItem("attempt", attemptSwitcher(t, at)));
    if (at.worker) meta.appendChild(metaItem("worker", document.createTextNode("#" + at.worker)));
    if (at.startedAt) meta.appendChild(metaItem("started", document.createTextNode(timeOnly(at.startedAt))));
    wrap.appendChild(meta);

    var main = el("div", {});
    var side = sidebar(t, at);
    wrap.appendChild(el("div", { class: "detail-grid" }, [main, side]));

    if (at.status === "fail") main.appendChild(assertionPanel(at));

    var counts = {
      screencast: (at.frames || []).length,
      screenshots: (at.artifacts || []).filter(function (a) { return a.kind === "image"; }).length,
      trace: (at.trace || []).length,
      logs: (at.artifacts || []).filter(function (a) { return a.kind === "log" && a.data; }).length,
    };
    var tabbar = el("div", { class: "tabs" });
    [["screencast", "Screencast"], ["screenshots", "Screenshots"], ["trace", "Trace"], ["logs", "Logs"]].forEach(function (tn) {
      var lbl = [tn[1]];
      if (counts[tn[0]]) lbl.push(el("span", { class: "n", text: String(counts[tn[0]]) }));
      tabbar.appendChild(el("div", {
        class: "tab" + (tab === tn[0] ? " active" : ""),
        onclick: function () { go(detailHash(t.name, tn[0], attN)); },
      }, lbl));
    });
    main.appendChild(tabbar);

    var panel = el("div", {});
    main.appendChild(panel);
    if (tab === "screencast") screencastTab(panel, at, t);
    else if (tab === "screenshots") screenshotsTab(panel, at);
    else if (tab === "trace") traceTab(panel, at, t);
    else logsTab(panel, at);
  }

  function detailHash(name, tab, attN) {
    var h = "test=" + encodeURIComponent(name) + "&tab=" + tab;
    if (attN > 0) h += "&att=" + attN;
    return h;
  }
  function metaItem(label, node) { return el("span", {}, [el("span", { text: label + " " }), wrapB(node)]); }
  function wrapB(node) { var b = el("b"); b.appendChild(node); return b; }
  function timeOnly(s) { var d = new Date(s); return isNaN(d) ? s : d.toLocaleTimeString(); }
  function attemptSwitcher(t, cur) {
    var box = el("b");
    box.appendChild(document.createTextNode((cur.n || 1) + " of " + t.attempts.length + "  "));
    t.attempts.forEach(function (a) {
      box.appendChild(el("button", { class: "pbtn" + (a.n === cur.n ? " on" : ""), onclick: function () { go(detailHash(t.name, currentTab(), a.n)); } }, [String(a.n)]));
      box.appendChild(document.createTextNode(" "));
    });
    return box;
  }
  function currentTab() { return parseHash().tab || "screencast"; }

  function assertionPanel(at) {
    var f = at.failure;
    var locTxt = f && f.location ? f.location : "";
    var body;
    if (f && (f.want || f.got || f.message)) {
      var lines = [];
      if (f.message) lines.push("expect: " + f.message);
      if (f.want) lines.push("  want: " + f.want);
      if (f.got) lines.push("  got:  " + f.got);
      if ((f.stack || []).length) { lines.push(""); (f.stack || []).forEach(function (s) { lines.push("  " + s.file + ":" + s.line + "  " + s.func); }); }
      body = lines.join("\n");
    } else {
      body = failureFromOutput(at.output) || "This test failed. See the trace and logs below.";
    }
    return el("div", { class: "assert" }, [
      el("div", { class: "ahead" }, [el("span", { text: "Assertion failed" }), el("span", { class: "loc", text: locTxt })]),
      el("pre", { text: body }),
    ]);
  }
  function failureFromOutput(out) {
    if (!out) return "";
    var keep = [];
    out.split("\n").forEach(function (ln) {
      var s = ln.trim();
      if (!s) return;
      if (/^(=== RUN|=== PAUSE|=== CONT|--- PASS|--- FAIL|ok\s|PASS$|FAIL$)/.test(s)) return;
      if (/^artifacts\.go:\d+: gantrytest: (screenshot|artifacts)/.test(s)) return;
      if (/^gantrytest\.go:\d+: gantrytest: screencast/.test(s)) return;
      keep.push(s);
    });
    return keep.slice(0, 40).join("\n");
  }

  // ---- screencast tab ----
  function screencastTab(panel, at, t) {
    var frames = at.frames || [];
    if (!frames.length) {
      panel.appendChild(el("div", { class: "empty", text: at.status === "skip" ? "This test was skipped no recording." : "No screencast for this test. Run with --record to capture one." }));
      return;
    }
    var canvas = el("canvas");
    var ctx = canvas.getContext("2d");
    var idx = 0, playing = false, speed = 1, raf = null, lastTs = 0, clock = 0, muted = false;
    var total = frames[frames.length - 1].t + 0.5;
    var imgs = frames.map(function (fr) { var im = new Image(); im.src = fr.data; return im; });

    imgs[0].onload = function () { canvas.width = imgs[0].naturalWidth; canvas.height = imgs[0].naturalHeight; draw(0); metaBadge.textContent = imgs[0].naturalWidth + "×" + imgs[0].naturalHeight + " · screencast.avi"; };
    if (imgs[0].complete && imgs[0].naturalWidth) imgs[0].onload();

    function draw(i) {
      idx = Math.max(0, Math.min(frames.length - 1, i));
      var im = imgs[idx];
      if (im.complete && im.naturalWidth) { canvas.width = im.naturalWidth; canvas.height = im.naturalHeight; ctx.drawImage(im, 0, 0); }
      scrub.value = String(idx);
      timeEl.textContent = frames[idx].t.toFixed(2) + " / " + total.toFixed(2);
      failBadge.classList.toggle("hidden", !(t.status === "fail" && idx === frames.length - 1));
      bigplay.classList.toggle("hidden", playing);
    }
    function tick(ts) {
      if (!playing) return;
      if (!lastTs) lastTs = ts;
      clock += (ts - lastTs) / 1000 * speed;
      lastTs = ts;
      var i = idx;
      while (i + 1 < frames.length && frames[i + 1].t <= clock) i++;
      draw(i);
      if (clock >= total) { pause(); return; }
      raf = requestAnimationFrame(tick);
    }
    function play() {
      if (idx >= frames.length - 1) { idx = 0; clock = 0; } else { clock = frames[idx].t; }
      playing = true; lastTs = 0; playBtn.replaceChildren(icon("pause")); bigplay.classList.add("hidden");
      raf = requestAnimationFrame(tick);
    }
    function pause() { playing = false; if (raf) cancelAnimationFrame(raf); playBtn.replaceChildren(icon("play")); bigplay.classList.remove("hidden"); }
    function toggle() { playing ? pause() : play(); }

    var failBadge = el("div", { class: "frame-badge hidden", text: "failure frame" });
    var metaBadge = el("div", { class: "meta-badge", text: "screencast.avi" });
    var bigplay = el("div", { class: "bigplay", onclick: toggle }, [icon("play", 26)]);
    var stage = el("div", { class: "stage" }, [canvas, metaBadge, failBadge, bigplay]);
    var scrub = el("input", { type: "range", class: "scrub", min: "0", max: String(frames.length - 1), value: "0",
      oninput: function () { pause(); idx = parseInt(this.value, 10); clock = frames[idx].t; draw(idx); } });
    var playBtn = el("button", { class: "pbtn", onclick: toggle }, [icon("play")]);
    var timeEl = el("span", { class: "time", text: "0.00 / " + total.toFixed(2) });
    var speedBtn = el("button", { class: "pbtn", text: "1×", onclick: function () { speed = speed === 1 ? 2 : speed === 2 ? 0.5 : 1; this.textContent = speed + "×"; } });
    var stepB = el("button", { class: "pbtn", onclick: function () { pause(); draw(idx - 1); clock = frames[idx].t; } }, [icon("left")]);
    var stepF = el("button", { class: "pbtn", onclick: function () { pause(); draw(idx + 1); clock = frames[idx].t; } }, [icon("right")]);
    var muteB = el("button", { class: "pbtn", onclick: function () { muted = !muted; this.replaceChildren(icon(muted ? "mute" : "volume")); } }, [icon("volume")]);
    var saveB = el("button", { class: "pbtn", onclick: function () { downloadDataURI(frames[idx].data, "frame-" + idx + ".jpg"); } }, [icon("download"), "save"]);
    var fsB = el("button", { class: "pbtn", onclick: function () { if (stage.requestFullscreen) stage.requestFullscreen(); } }, [icon("maximize")]);

    panel.appendChild(el("div", { class: "player" }, [
      stage, scrub,
      el("div", { class: "ctrls" }, [playBtn, timeEl, el("span", { class: "spacer" }), speedBtn, stepB, stepF, muteB, saveB, fsB]),
    ]));
  }

  // ---- screenshots tab ----
  function screenshotsTab(panel, at) {
    var shots = (at.artifacts || []).filter(function (a) { return a.kind === "image"; });
    if (!shots.length) { panel.appendChild(el("div", { class: "empty", text: "No screenshots for this test." })); return; }
    var stageImg = el("img");
    var toolbar = el("div", { class: "shot-toolbar" }, []);
    var stage = el("div", { class: "shot-stage" }, [stageImg]);
    function show(i) {
      var a = shots[i];
      stageImg.src = a.data;
      clear(toolbar);
      toolbar.appendChild(el("span", { class: "mono", text: a.name }));
      toolbar.appendChild(el("span", { text: " · " + fmtBytes(a.size) }));
      toolbar.appendChild(el("span", { class: "spacer" }));
      toolbar.appendChild(el("button", { class: "pbtn", onclick: function () { downloadDataURI(a.data, a.name); } }, [icon("download"), "save"]));
      toolbar.appendChild(el("button", { class: "pbtn", onclick: function () { if (stage.requestFullscreen) stage.requestFullscreen(); } }, [icon("maximize")]));
      Array.prototype.forEach.call(thumbs.children, function (c, j) { c.classList.toggle("sel", j === i); });
    }
    panel.appendChild(el("div", {}, [toolbar, stage]));
    var thumbs = el("div", { class: "thumbs" });
    shots.forEach(function (a, i) {
      thumbs.appendChild(el("div", { class: "thumb", onclick: function () { show(i); } }, [
        el("img", { src: a.data }), el("div", { class: "nm", text: a.name }), el("div", { class: "cap", text: a.desc || "" }),
      ]));
    });
    panel.appendChild(thumbs);
    show(0);
  }

  // ---- trace tab ----
  function traceTab(panel, at, t) {
    var entries = at.trace || [];
    if (!entries.length) { panel.appendChild(el("div", { class: "empty", text: "No trace for this test." })); return; }
    var maxT = entries[entries.length - 1].t || 0.001;
    var state = { q: "", dir: "all" };

    var counts = { all: entries.length, action: 0, recv: 0, send: 0 };
    entries.forEach(function (e) { if (counts[e.dir] != null) counts[e.dir]++; });
    var chipbox = el("div", { class: "chips" });
    [["all", "all"], ["action", "action"], ["recv", "recv"], ["send", "send"]].forEach(function (d) {
      chipbox.appendChild(el("div", { class: "chip" + (state.dir === d[0] ? " active" : ""), "data-k": d[0],
        onclick: function () { state.dir = d[0]; refreshChips(); redraw(); } }, [d[1], el("span", { class: "n", text: String(counts[d[0]]) })]));
    });
    function refreshChips() { Array.prototype.forEach.call(chipbox.children, function (c) { c.classList.toggle("active", c.getAttribute("data-k") === state.dir); }); }
    var input = el("input", { placeholder: "t:render  seq>40  free text", oninput: function () { state.q = this.value; redraw(); } });
    panel.appendChild(el("div", { class: "trace-controls" }, [chipbox, el("div", { class: "search" }, [icon("search"), input])]));

    // timeline with a draggable playhead scrubber
    var labels = el("div", { class: "tl-labels" }, ["action", "recv", "send"].map(function (d) { return el("div", { class: "tl-lbl", text: d }); }));
    var tracks = el("div", { class: "tl-tracks" });
    ["action", "recv", "send"].forEach(function (dir) {
      var track = el("div", { class: "tl-track" });
      entries.filter(function (e) { return e.dir === dir; }).forEach(function (e) {
        track.appendChild(el("div", { class: "tl-tick " + dir, style: "left:" + (100 * e.t / maxT) + "%", title: "+" + e.t.toFixed(3) + "s" }));
      });
      tracks.appendChild(track);
    });
    var playhead = el("div", { class: "tl-playhead", style: "left:0%" }, [el("div", { class: "tl-handle" })]);
    tracks.appendChild(playhead);
    var endMark = el("div", { class: "tl-endmark", style: "left:100%" }, [
      t.status === "fail" ? el("span", { class: "tl-endlabel", text: "expect failed" }) : null,
    ]);
    tracks.appendChild(endMark);

    var lanes = el("div", { class: "tl-lanes" }, [labels, tracks]);
    var axis = el("div", { class: "tl-axis" }, [el("span", { text: "0.0s" }), el("span", { text: (maxT / 2).toFixed(1) + "s" }), el("span", { text: maxT.toFixed(1) + "s" })]);
    panel.appendChild(el("div", { class: "tl" }, [lanes, axis]));

    // scrubbing: drag anywhere on the tracks to move the playhead and
    // highlight the nearest frame in the table below.
    var dragging = false;
    function seekFromX(clientX) {
      var r = tracks.getBoundingClientRect();
      var pct = Math.max(0, Math.min(1, (clientX - r.left) / r.width));
      playhead.style.left = (pct * 100) + "%";
      highlightNearest(pct * maxT);
    }
    tracks.addEventListener("mousedown", function (e) { dragging = true; seekFromX(e.clientX); e.preventDefault(); });
    window.addEventListener("mousemove", function (e) { if (dragging) seekFromX(e.clientX); });
    window.addEventListener("mouseup", function () { dragging = false; });

    // table
    var tbody = el("tbody");
    panel.appendChild(el("table", { class: "trace" }, [
      el("thead", {}, [el("tr", {}, [el("th", { text: "time" }), el("th", { text: "dir" }), el("th", { text: "msg / frame" }), el("th", { class: "seq", text: "seq" })])]),
      tbody,
    ]));

    var rowsByEntry = [];
    function highlightNearest(tSec) {
      var best = -1, bestD = 1e9;
      entries.forEach(function (e, i) { if (!isFiltered(e)) return; var d = Math.abs(e.t - tSec); if (d < bestD) { bestD = d; best = i; } });
      rowsByEntry.forEach(function (r) { if (r) r.classList.remove("cursor"); });
      if (best >= 0 && rowsByEntry[best]) { rowsByEntry[best].classList.add("cursor"); rowsByEntry[best].scrollIntoView({ block: "nearest" }); }
    }
    function isFiltered(e) { return matches(e); }
    function matches(e) {
      if (state.dir !== "all" && e.dir !== state.dir) return false;
      var terms = state.q.trim().toLowerCase().split(/\s+/).filter(Boolean);
      for (var i = 0; i < terms.length; i++) {
        var term = terms[i], m;
        if (term.indexOf("t:") === 0) { if (!(e.frame && (e.frame.t || "").toLowerCase().indexOf(term.slice(2)) >= 0)) return false; }
        else if ((m = term.match(/^seq([<>]=?)(\d+)$/))) {
          var seq = e.frame ? e.frame.seq || 0 : 0, v = parseInt(m[2], 10);
          if (m[1] === ">" && !(seq > v)) return false;
          if (m[1] === "<" && !(seq < v)) return false;
          if (m[1] === ">=" && !(seq >= v)) return false;
          if (m[1] === "<=" && !(seq <= v)) return false;
        } else if (summary(e).toLowerCase().indexOf(term) < 0) return false;
      }
      return true;
    }
    function redraw() {
      clear(tbody);
      rowsByEntry = [];
      entries.forEach(function (e, i) { if (matches(e)) rowsByEntry[i] = addRow(tbody, e); });
    }
    function addRow(tbody, e) {
      var row = el("tr", { class: "tr-row" }, [
        el("td", { class: "tcol", text: "+" + e.t.toFixed(3) + "s" }),
        el("td", {}, [el("span", { class: "dirtag " + e.dir, text: e.dir })]),
        el("td", { class: "msg", text: summary(e) }),
        el("td", { class: "seq", text: e.frame && e.frame.seq ? String(e.frame.seq) : "" }),
      ]);
      var open = false, detail = null;
      row.addEventListener("click", function () {
        open = !open;
        if (open) { detail = detailRow(e); tbody.insertBefore(detail, row.nextSibling); }
        else if (detail) { tbody.removeChild(detail); detail = null; }
      });
      tbody.appendChild(row);
      return row;
    }
    redraw();
  }

  function summary(e) {
    if (e.dir === "action") return e.msg || "";
    var f = e.frame; if (!f) return "";
    var s = f.t || "";
    if (f.key) s += " " + f.key;
    if (f.name) s += "." + f.name;
    if (f.code) s += " [" + f.code + "]";
    return s;
  }
  function detailRow(e) {
    var showTree = false;
    var pre = el("pre", { text: e.raw || e.msg || "" });
    var bar = el("div", { class: "subbar" });
    if (e.frame && e.frame.tree) {
      bar.appendChild(el("button", { class: "pbtn", text: "raw", onclick: function () { showTree = !showTree; pre.textContent = showTree ? e.frame.tree : e.raw; this.textContent = showTree ? "tree" : "raw"; } }));
    }
    bar.appendChild(el("button", { class: "pbtn", onclick: function () { copy(e.raw || e.msg || "", "Line copied"); } }, [icon("copy"), "copy line"]));
    if (e.time) bar.appendChild(el("span", { class: "mono", style: "color:var(--faint);align-self:center", text: e.time }));
    return el("tr", { class: "tr-detail" }, [el("td", { colspan: "4" }, [bar, pre])]);
  }

  // ---- logs tab ----
  function logsTab(panel, at) {
    var logs = (at.artifacts || []).filter(function (a) { return a.kind === "log" && a.data; });
    if (!logs.length) { panel.appendChild(el("div", { class: "empty", text: "No logs for this test." })); return; }
    var tabs = el("div", { class: "log-tabs" });
    var pre = el("pre", { class: "log" });
    logs.forEach(function (a, i) {
      tabs.appendChild(el("div", { class: "log-tab" + (i === 0 ? " active" : ""), text: a.name, onclick: function () {
        pre.textContent = a.data; Array.prototype.forEach.call(tabs.children, function (c, j) { c.classList.toggle("active", j === i); });
      } }));
    });
    pre.textContent = logs[0].data;
    panel.appendChild(tabs);
    panel.appendChild(pre);
  }

  // ---- sidebar ----
  var ARTIFACT_ICON = { image: "image", video: "film", log: "file", trace: "activity" };
  function sidebar(t, at) {
    var arts = (at.artifacts || []).slice();
    var totalBytes = arts.reduce(function (s, a) { return s + (a.size || 0); }, 0);
    var box = el("div", { class: "sidebar" });
    box.appendChild(el("div", { class: "shead" }, [el("span", { text: "Artifacts" }), el("span", { class: "n", text: arts.length + " files · " + fmtBytes(totalBytes) })]));
    arts.forEach(function (a) {
      box.appendChild(el("div", { class: "art", onclick: function () { openArtifact(t, a); } }, [
        el("span", { class: "ico" }, [icon(ARTIFACT_ICON[a.kind] || "file")]),
        el("div", { class: "info" }, [el("div", { class: "nm", text: a.name }), el("div", { class: "ds", text: a.desc || "" })]),
        el("span", { class: "sz", text: fmtBytes(a.size) }),
      ]));
    });
    if (!arts.some(function (a) { return a.name === "crash.log"; })) {
      box.appendChild(el("div", { class: "art absent" }, [
        el("span", { class: "ico" }, [icon("alert")]),
        el("div", { class: "info" }, [el("div", { class: "nm", text: "crash.log" }), el("div", { class: "ds", text: "not produced process exited cleanly" })]),
        el("span", { class: "sz", text: "–" }),
      ]));
    }
    if (t.status === "fail" || t.status === "flaky") {
      box.appendChild(el("div", { class: "note" }, [icon("info"), el("span", { text: "Kept because this test " + (t.status === "flaky" ? "was flaky" : "failed") + ". Passing tests discard artifacts unless --keep-artifacts is set." })]));
    }
    if (at.dir) box.appendChild(el("div", { class: "outdir" }, [el("div", { text: "Output directory" }), el("div", { class: "p", text: at.dir + "/" })]));
    return box;
  }
  function openArtifact(t, a) {
    var attN = parseInt(parseHash().att || "0", 10);
    var tab = a.kind === "video" ? "screencast" : a.kind === "image" ? "screenshots" : a.kind === "trace" ? "trace" : a.kind === "log" ? "logs" : null;
    if (tab) go(detailHash(t.name, tab, attN));
  }

  // ---- utilities ----
  function runPattern(name) {
    return name.split("/").map(function (p) { return "^" + p.replace(/[.*+?()|\[\]{}^$\\]/g, "\\$&") + "$"; }).join("/");
  }
  function copy(text, msg) {
    if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(text).then(function () { toast(msg); }, function () { toast("Copy failed"); });
    else { var ta = document.createElement("textarea"); ta.value = text; document.body.appendChild(ta); ta.select(); try { document.execCommand("copy"); toast(msg); } catch (e) { toast("Copy failed"); } document.body.removeChild(ta); }
  }
  function downloadDataURI(uri, name) { var a = document.createElement("a"); a.href = uri; a.download = name; document.body.appendChild(a); a.click(); document.body.removeChild(a); }
  var toastEl = null, toastTimer = null;
  function toast(msg) {
    if (!toastEl) { toastEl = el("div", { class: "toast" }); document.body.appendChild(toastEl); }
    toastEl.textContent = msg; toastEl.classList.add("show");
    clearTimeout(toastTimer); toastTimer = setTimeout(function () { toastEl.classList.remove("show"); }, 1600);
  }

  render();
})();
