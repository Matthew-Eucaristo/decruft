// decruft site: an interactive replica of the TUI, plus small page helpers.
// Plain JS, no dependencies, no network. The numbers are the recorded demo's
// synthetic machine (demo/setup.sh), so nothing here describes a real disk.
(() => {
  "use strict";
  document.documentElement.classList.add("js");
  const reduceMotion = matchMedia("(prefers-reduced-motion: reduce)").matches;
  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => [...el.querySelectorAll(s)];

  /* ---------- copy buttons ---------- */
  $$("[data-copy]").forEach((b) => b.addEventListener("click", async () => {
    const text = b.dataset.copy;
    try { await navigator.clipboard.writeText(text); }
    catch {
      const t = Object.assign(document.createElement("textarea"), { value: text });
      document.body.append(t); t.select(); document.execCommand("copy"); t.remove();
    }
    b.classList.add("done");
    clearTimeout(b._t); b._t = setTimeout(() => b.classList.remove("done"), 1600);
  }));

  /* ---------- install tabs (ARIA tabs pattern) ---------- */
  const tabs = $$('[role="tab"]');
  const select = (t) => {
    tabs.forEach((x) => {
      const on = x === t;
      x.setAttribute("aria-selected", on); x.tabIndex = on ? 0 : -1;
      $("#" + x.getAttribute("aria-controls")).hidden = !on;
    });
    t.focus();
  };
  tabs.forEach((t, i) => {
    t.addEventListener("click", () => select(t));
    t.addEventListener("keydown", (e) => {
      const d = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
      if (d) { e.preventDefault(); select(tabs[(i + d + tabs.length) % tabs.length]); }
    });
  });

  /* ---------- reveal on scroll ---------- */
  if (!reduceMotion && "IntersectionObserver" in window) {
    const io = new IntersectionObserver((es) => es.forEach((e) => {
      if (e.isIntersecting) { e.target.classList.add("in"); io.unobserve(e.target); }
    }), { rootMargin: "0px 0px -8% 0px" });
    $$(".sec h2, .sec .intro, .card, .cat, .safety article, .kid, .compare, .checks li, .recording, .receipt, .platforms div, .usage").forEach((el, i) => {
      el.classList.add("reveal"); el.style.transitionDelay = (i % 4) * 60 + "ms"; io.observe(el);
    });
  }

  /* ---------- recording: video, GIF fallback ---------- */
  const video = $("#demo-video"), play = $("#play");
  if (video) {
    const toGif = () => {
      const img = Object.assign(new Image(1160, 620), { src: "demo.gif", alt: video.getAttribute("aria-label"), loading: "lazy" });
      video.replaceWith(img); play.hidden = true;
    };
    const srcs = $$("source", video);
    srcs[srcs.length - 1].addEventListener("error", toGif);
    const start = (manual) => { play.hidden = true; if (manual === true) video.controls = true; video.play().catch(() => { play.hidden = false; }); };
    play.addEventListener("click", () => start(true));
    video.addEventListener("click", () => { if (!video.controls) video.paused ? video.play() : video.pause(); });
    const saveData = navigator.connection && navigator.connection.saveData;
    if (!reduceMotion && !saveData && "IntersectionObserver" in window) {
      const vio = new IntersectionObserver((es) => es.forEach((e) => {
        if (e.isIntersecting) start(); else if (!video.paused) video.pause();
      }), { threshold: 0.5 });
      vio.observe(video);
    }
  }

  /* =====================================================================
     The TUI replica
     ===================================================================== */
  const term = $("#term");
  if (!term) return;
  const keysEl = $("#term-keys"), hint = $("#try-hint");

  const MB = 1e6, GB = 1e9;
  const DISK = 6.2 * GB;
  const FREE0 = 1.8 * GB;
  const T = (tool) => ({ kind: "family", tool, how: "→ " + tool });
  const C = (cmd) => ({ kind: "cmd", how: "via " + cmd, cmd });
  const TRASH = { kind: "trash", how: "→ Trash" };

  const CATS = [
    { name: "Xcode & Simulators", color: "#6E7CF5", note: "↺ regrows", items: [
      { name: "iOS 26.4 simulator runtime", size: 1249.0 * MB, level: "review", ...C("xcrun simctl runtime delete A1B2C3D4-0000-4000-8000-000000000264"),
        path: "/Library/Developer/CoreSimulator/Volumes/iOS_23E244", what: "A simulator runtime. Several GB each, hidden inside “System Data”.", back: "Xcode › Settings › Components downloads it again", why: "you may still test on this iOS version" },
      { name: "DerivedData · ShopApp", size: 370.1 * MB, level: "safe", ...TRASH,
        path: "~/Library/Developer/Xcode/DerivedData/ShopApp-bqzdlxkfmcvnwhatyrpeoiusgjxa", what: "Build products, indexes and logs for one project. Xcode rebuilds them.", back: "the next build in Xcode (slower once, re-indexes)", why: "a cache Xcode recreates" },
      { name: "Unavailable simulators (1)", size: 230.7 * MB, level: "safe", ...C("xcrun simctl delete unavailable"),
        path: "~/Library/Developer/CoreSimulator/Devices", what: "Simulators whose runtime is gone. They can never boot again.", back: "never needed", why: "unusable by definition" },
      { name: "iOS DeviceSupport · iPhone15,2 17.5 (21F79)", size: 188.7 * MB, level: "review", ...TRASH,
        path: "~/Library/Developer/Xcode/iOS DeviceSupport/iPhone15,2 17.5 (21F79)", what: "Symbols copied from a device to debug on it.", back: "copied again when that device connects", why: "the copy takes a while on the next connect" },
      { name: "DerivedData · WeatherKit", size: 157.3 * MB, level: "safe", ...TRASH,
        path: "~/Library/Developer/Xcode/DerivedData/WeatherKit-cfhqzmrxpltkwnsdyjgaobveiu", what: "Build products, indexes and logs for one project. Xcode rebuilds them.", back: "the next build in Xcode", why: "a cache Xcode recreates" },
      { name: "DerivedData · Module cache", size: 94.4 * MB, level: "safe", ...TRASH,
        path: "~/Library/Developer/Xcode/DerivedData/ModuleCache.noindex", what: "Precompiled module cache shared by projects.", back: "the next build", why: "a cache Xcode recreates" },
      { name: "Simulator · iPhone 17 (iOS 26.4)", size: 73.4 * MB, level: "review", ...C("xcrun simctl delete 9B2C4D6E-2222-4F70-8A1B-2C3D4E5F6A7B"),
        path: "~/Library/Developer/CoreSimulator/Devices/9B2C4D6E-…", what: "One simulator device with its installed apps and data.", back: "create it again in Xcode › Devices", why: "its apps and data go with it" },
      { name: "Archives", size: 62.9 * MB, level: "protected", kind: "none", how: "remove it in the app",
        path: "~/Library/Developer/Xcode/Archives", what: "App archives, with the dSYMs that symbolicate crash reports.", back: "they do not come back", why: "Xcode › Organizer is the owner's way to remove them" },
    ] },
    { name: "Go", color: "#2EC4B6", note: "↺ regrows", items: [
      { name: "Go build cache", size: 629.1 * MB, level: "safe", ...C("go clean -cache"), path: "~/Library/Caches/go-build", what: "Compiled packages and test results.", back: "the next go build", why: "a cache Go recreates" },
      { name: "Go module cache", size: 367.1 * MB, level: "safe", ...C("go clean -modcache"), path: "~/go/pkg/mod", what: "Downloaded module sources (read-only; plain rm fails).", back: "the next go build downloads what it needs", why: "a cache Go recreates" },
    ] },
    { name: "Rust & Cargo", color: "#D9483B", note: "via rustkill", items: [
      { name: "shop-api · target", size: 272.6 * MB, level: "safe", ...T("rustkill"), path: "~/dev/shop-api/target", what: "Cargo's build output: compiled dependencies, incremental caches and the binaries in debug/ and release/.", back: "cargo build (from scratch, so the first build is slow)", why: "rustkill's rules: cargo's own tag, git-ignored, idle" },
      { name: "toolchain 1.80.0", size: 125.8 * MB, level: "review", ...T("rustkill"), path: "~/.rustup/toolchains/1.80.0-aarch64-apple-darwin", what: "A Rust toolchain rustup installed: rustc, cargo and the standard library. rustkill runs rustup toolchain uninstall.", back: "rustup toolchain install 1.80.0", why: "no project under the scanned folders pins it, but check" },
      { name: "desk-app · target", size: 125.8 * MB, level: "safe", ...T("rustkill"), path: "~/dev/desk-app/src-tauri/target", what: "A Tauri app's Rust build output. nodekill sees it too; it is shown once, as rustkill's.", back: "cargo build / tauri dev", why: "rustkill's rules: cargo's own tag, git-ignored, idle" },
      { name: "crates.io sources", size: 62.9 * MB, level: "safe", ...T("rustkill"), path: "~/.cargo/registry/src/index.crates.io-1949cf8c6b5b557f", what: "Crates Cargo unpacked from its downloads so rustc can compile them.", back: "cargo unpacks them again from registry/cache, offline", why: "rustkill's rules: Cargo restores what a build needs" },
      { name: "toolchain stable (1.91.0)", size: 42.0 * MB, level: "protected", kind: "none", how: "rustkill leaves this alone", path: "~/.rustup/toolchains/stable-aarch64-apple-darwin", what: "rustup's default toolchain.", back: "rustup toolchain install stable", why: "rustup's default toolchain is never offered" },
      { name: "crates.io downloads", size: 36.7 * MB, level: "review", ...T("rustkill"), path: "~/.cargo/registry/cache/index.crates.io-1949cf8c6b5b557f", what: "The .crate archives Cargo downloaded.", back: "cargo downloads the crates again (builds need the network once)", why: "removing it means downloading again" },
    ] },
    { name: "Homebrew", color: "#F59E42", note: "↺ regrows", items: [
      { name: "Old versions, downloads and logs", size: 280.0 * MB, level: "safe", ...C("brew cleanup --prune=all"), path: "~/Library/Caches/Homebrew", what: "Outdated kegs, cached downloads and logs.", back: "re-downloaded only if you reinstall", why: "brew's own cleanup" },
      { name: "Unused dependencies", size: 188.7 * MB, level: "review", ...C("brew autoremove"), path: "/opt/homebrew/Cellar", what: "Formulae installed as dependencies that nothing needs now.", back: "brew install them again", why: "uninstalls formulae; offered only when none provides a command on your PATH" },
    ] },
    { name: "Node.js", color: "#E8C93A", note: "via nodekill", items: [
      { name: "shop-web · node_modules", size: 272.6 * MB, level: "safe", ...T("nodekill"), path: "~/dev/shop-web/node_modules", what: "Installed dependencies of one project.", back: "npm install", why: "nodekill's rules: not touched recently" },
      { name: "admin-panel · .next cache", size: 125.9 * MB, level: "safe", ...T("nodekill"), path: "~/dev/admin-panel/.next", what: "Next.js build output and cache.", back: "the next build", why: "nodekill's rules: a framework cache" },
    ] },
    { name: "Package caches", color: "#B48CF2", note: "", items: [
      { name: "CocoaPods cache", size: 141.2 * MB, level: "safe", ...C("pod cache clean --all"), path: "~/Library/Caches/CocoaPods", what: "Downloaded pod sources.", back: "the next pod install", why: "a cache CocoaPods recreates" },
      { name: "uv cache", size: 88.9 * MB, level: "safe", ...C("uv cache prune"), path: "~/.cache/uv", what: "Unused wheels and sources.", back: "the next uv sync", why: "prune keeps what is in use" },
      { name: "pip cache", size: 42.5 * MB, level: "safe", ...C("pip3 cache purge"), path: "~/Library/Caches/pip", what: "Downloaded wheels.", back: "the next pip install", why: "a cache pip recreates" },
    ] },
    { name: "Flutter & Dart", color: "#4CC3F2", note: "via flutterkill", items: [
      { name: "weather_app · build", size: 171.4 * MB, level: "safe", ...T("flutterkill"), path: "~/dev/weather_app/build", what: "Flutter build outputs.", back: "flutter build", why: "flutterkill's rules" },
      { name: "shop_mobile · .dart_tool", size: 98.1 * MB, level: "safe", ...T("flutterkill"), path: "~/dev/shop_mobile/.dart_tool", what: "Dart tool state and build cache.", back: "flutter pub get", why: "flutterkill's rules" },
    ] },
    { name: "Git worktrees", color: "#C9A27A", note: "via fell", items: [
      { name: "shop-web · agent/checkout-flow", size: 157.3 * MB, level: "review", ...T("fell"), path: "~/dev/.worktrees/shop-web/checkout-flow", what: "An AI-agent worktree, merged two weeks ago.", back: "git worktree add, from the branch", why: "fell's rules: clean, merged, but yours to confirm" },
    ] },
  ];
  CATS.forEach((c) => c.items.forEach((it) => { it.cat = c; it.size0 = it.size; }));
  const ALL = CATS.flatMap((c) => c.items);
  const selectable = (it) => !it.done && (it.level === "safe" || it.level === "review");

  /* ---------- helpers ---------- */
  const esc = (s) => String(s).replace(/[&<>"]/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[ch]));
  const fmt = (b) => b <= 0 ? "0 B" : b < GB ? (b / MB).toFixed(1) + " MB" : (b / GB).toFixed(1) + " GB";
  const trunc = (s, n) => (n <= 0 ? "" : s.length > n ? s.slice(0, Math.max(0, n - 1)) + "…" : s);
  const pad = (s, n) => { s = trunc(s, n); return s + " ".repeat(Math.max(0, n - s.length)); };
  const padL = (s, n) => { s = trunc(s, n); return " ".repeat(Math.max(0, n - s.length)) + s; };
  const sp = (cls, s) => `<span class="${cls}">${esc(s)}</span>`;
  const sw = (color) => `<span class="sw" style="background:${color}"></span>`;
  const sum = (a, f) => a.reduce((t, x) => t + f(x), 0);
  const left = (c) => c.items.filter((i) => !i.done);
  const catSize = (c) => sum(left(c), (i) => i.size);
  const catLv = (c, lv) => sum(left(c).filter((i) => i.level === lv), (i) => i.size);
  const icon = { safe: "●", review: "▲", protected: "■", live: "◉" };
  const lvCls = { safe: "g", review: "y", protected: "r", live: "m" };
  const badge = (lv) => sp(lvCls[lv], pad(icon[lv] + " " + lv, 11));

  /* ---------- springs (harmonica-style damped oscillator) ---------- */
  const springs = new Map();
  const spring = (key, target) => {
    let s = springs.get(key);
    if (!s) { s = { x: target, v: 0, t: target }; springs.set(key, s); }
    s.t = target;
    return s;
  };
  let raf = 0;
  const animate = () => {
    if (raf) return;
    let last = performance.now();
    const step = (now) => {
      const dt = Math.min(0.033, (now - last) / 1000); last = now;
      let moving = false;
      const w = 8.5, z = 0.62; // angular frequency, damping ratio
      for (const s of springs.values()) {
        if (reduceMotion) { s.x = s.t; s.v = 0; continue; }
        const a = -w * w * (s.x - s.t) - 2 * z * w * s.v;
        s.v += a * dt; s.x += s.v * dt;
        if (Math.abs(s.x - s.t) > 1e-4 || Math.abs(s.v) > 1e-4) moving = true; else { s.x = s.t; s.v = 0; }
      }
      paintSprings();
      raf = moving ? requestAnimationFrame(step) : 0;
    };
    raf = requestAnimationFrame(step);
  };
  const paintSprings = () => {
    $$("[data-spring]", term).forEach((el) => {
      const s = springs.get(el.dataset.spring);
      if (s) el.style[el.dataset.prop || "width"] = Math.max(0, s.x * 100) + "%";
    });
  };

  /* ---------- state ---------- */
  const st = { view: "overview", cur: 0, icur: 0, tcur: 0, zoom: null, free: FREE0, cleaned: false, steps: [], receipt: null, help: false };
  let cols = 120, rows = 30;

  const measure = () => {
    const probe = document.createElement("span");
    probe.textContent = "0".repeat(100);
    probe.style.cssText = "position:absolute;visibility:hidden;white-space:pre";
    term.append(probe);
    const cw = probe.getBoundingClientRect().width / 100;
    const lh = parseFloat(getComputedStyle(term).lineHeight) || 18;
    probe.remove();
    const cs = getComputedStyle(term);
    const inner = term.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
    cols = Math.max(36, Math.floor(inner / cw));
    rows = Math.max(20, Math.round((term.clientHeight - parseFloat(cs.paddingTop) - parseFloat(cs.paddingBottom)) / lh));
  };

  const visibleCats = () => CATS.filter((c) => c.items.length);
  const selected = () => ALL.filter((i) => i.sel && !i.done);
  const found = () => sum(ALL, (i) => i.size0);
  const removable = () => sum(ALL.filter((i) => i.level !== "protected"), (i) => i.size0);

  /* ---------- shared chrome ---------- */
  const header = () => {
    const segs = visibleCats().filter((c) => catSize(c) > 0);
    const used = sum(segs, catSize);
    const total = used + st.free;
    // order: categories by original size, then free
    let bar = '<div class="sbar" role="img" aria-label="Storage bar">';
    for (const c of CATS) {
      const s = spring("bar:" + c.name, catSize(c) / total);
      bar += `<span data-spring="bar:${esc(c.name)}" style="width:${s.x * 100}%;background:${c.color}"></span>`;
    }
    bar += "</div>";
    const legend = segs.map((c) => `<span>${sw(c.color)}${sp("b", c.name)} ${sp("f", fmt(catSize(c)))}</span>`).join("") +
      `<span>${sw("#2a2a2a")}${sp("f", "free " + fmt(st.free))}</span>`;
    return `<span class="ln"><span class="brandb">decruft</span> ${sp("g", "● trash")} ${sp("f", "·")}  ${sp("b", fmt(st.free))} ${sp("f", "free of " + fmt(DISK) + "  ·  demo")}</span>` +
      bar + `<div class="legend">${legend}</div>`;
  };

  const status = (hints) => {
    const sel = selected();
    const fr = sum(sel, (i) => i.size);
    let s = `${sp("b", sel.length + " selected · frees " + fmt(fr))}  ${sp("f", "·  " + fmt(found()) + " found, " + fmt(removable()) + " removable here")}`;
    if (st.cleaned) s += `  ${sp("f", "·")}  ${sp("g", "free space +" + fmt(st.free - FREE0))} ${sp("f", "measured")}`;
    return `<span class="ln">${s}</span><span class="ln f">${esc(trunc(hints, cols))}</span>`;
  };

  const lines = (n) => '<span class="ln"> </span>'.repeat(Math.max(0, n));

  /* ---------- views ---------- */
  const overview = () => {
    const cats = visibleCats();
    st.cur = Math.min(st.cur, cats.length - 1);
    const max = Math.max(...cats.map((c) => c.items.reduce((t, i) => t + i.size0, 0)));
    const wide = cols >= 100, mid = cols >= 72;
    const nameW = 19, sizeW = 9, barW = wide ? 24 : mid ? 12 : 0;
    let out = "";
    let head = "     " + pad("CATEGORY", nameW + 2) + pad("FREES", sizeW + 1) + " ".repeat(barW ? barW + 2 : 0);
    if (mid) head += padL("SAFE", 10) + padL("REVIEW", 10);
    if (wide) head += "  WHAT";
    out += `<span class="ln f">${esc(head)}</span>`;
    cats.forEach((c, i) => {
      const on = i === st.cur && st.view === "overview";
      const items = left(c), sz = catSize(c), done = items.length === 0;
      const selN = items.filter((x) => x.sel).length, selAll = selN && selN === items.filter(selectable).length;
      const mark = done ? sp("g", "✓") : selAll ? sp("m", "●") : selN ? sp("m", "◐") : sp("f", "○");
      let ln = (on ? '<span class="cur">▌</span>' : " ") + " " + mark + " " + sw(c.color);
      if (done) {
        ln += sp("s", pad(c.name, nameW)) + "  " + sp("g", "cleaned");
      } else {
        ln += sp(on ? "b" : "", pad(c.name, nameW)) + "  " + sp("b", padL(fmt(sz), sizeW)) + " ";
        if (barW) {
          const w = Math.max(0.6, (sz / max) * barW);
          ln += ` <span class="hcell" style="width:${barW}ch"><span class="hbar" style="width:${w}ch;background:${c.color}"></span></span>  `;
        }
        if (mid) {
          const sf = catLv(c, "safe"), rv = catLv(c, "review");
          ln += sp("g", padL(sf ? fmt(sf) : "–", 10)) + sp("y", padL(rv ? fmt(rv) : "–", 10));
        }
        if (wide) {
          const n = items.length;
          ln += "  " + sp("f", n + (n === 1 ? " item" : " items")) + (c.note ? sp("f", " · ") + sp(c.note.startsWith("↺") ? "c" : "f", c.note) : "");
        }
      }
      out += `<span class="row ln${on ? " on" : ""}" data-cat="${i}">${ln}</span>`;
    });
    out += lines(1);
    const wins = ALL.filter((i) => i.level === "safe" && !i.done).sort((a, b) => b.size - a.size).slice(0, rows >= 30 ? 5 : 3);
    if (wins.length) {
      out += `<span class="ln">${sp("m b", "Biggest safe wins")}  ${sp("f", "·  s selects every safe item")}</span>`;
      wins.forEach((w) => {
        const nW = Math.min(36, cols - 34);
        out += `<span class="ln">  ${w.sel ? sp("m", "●") : sp("f", "○")} ${sw(w.cat.color)} ${sp("b", padL(fmt(w.size), 9))}  ${esc(pad(w.name, nW))}  ${sp("f", trunc(w.how, Math.max(0, cols - nW - 20)))}</span>`;
      });
      out += lines(1);
    }
    out += `<span class="ln">${sp("m b", "What macOS does not show you")}</span>`;
    out += `<span class="ln f">${esc(trunc("Inside “System Data”: simulator runtimes 1.2 GB.", cols))}</span>`;
    out += lines(1);
    const src = [["✓", "xcode"], ["✓", "homebrew"], ["✓", "go"], ["✓", "packages"], ["·", "docker"], ["·", "android"], ["✓", "fell 0.1.2"], ["✓", "rustkill 0.1.0"], ["✓", "nodekill 0.1.0"], ["✓", "flutterkill 0.3.0"]];
    let srcLine = sp("f", "Sources ");
    let len = 8;
    for (const [m, n] of src) {
      if (len + n.length + 4 > cols) break;
      srcLine += " " + (m === "✓" ? sp("g", "✓") + " " + sp("b", n) : sp("f", "· " + n)) + " ";
      len += n.length + 4;
    }
    out += `<span class="ln">${srcLine}</span>`;
    return { body: out, foot: status("↑↓ move  enter open  space select  s select safe  c clean  t treemap  R reset  ? help") };
  };

  const list = () => {
    const c = visibleCats()[st.cur];
    const items = c.items;
    st.icur = Math.min(st.icur, items.length - 1);
    const max = Math.max(...items.map((i) => i.size0));
    const barW = cols >= 80 ? 12 : 0;
    let out = `<span class="ln">${sw(c.color)}${sp("m b", c.name)}  ${sp("b", fmt(catSize(c)))}  ${sp("f", left(c).length + " items")}</span>`;
    out += `<span class="ln f">${esc("   " + " ".repeat(barW ? barW + 1 : 0) + padL("FREES", 9) + "  " + pad("SAFETY", 11) + " ITEM")}</span>`;
    items.forEach((it, i) => {
      const on = i === st.icur;
      const mark = it.done ? sp("g", "✓") : it.sel ? sp("m", "●") : sp("f", "○");
      let ln = (on ? '<span class="cur">▌</span>' : " ") + " " + mark + " ";
      if (barW) {
        const w = Math.max(0.4, (it.size0 / max) * barW);
        ln += `<span class="hcell" style="width:${barW}ch"><span class="hbar" style="width:${w}ch;background:${c.color};opacity:${it.done ? 0.25 : 1}"></span></span> `;
      }
      const rest = cols - (barW ? barW + 1 : 0) - 26;
      const nW = Math.min(it.name.length, Math.max(10, Math.floor(rest * 0.6)));
      if (it.done) {
        ln += sp("f", padL(fmt(it.size0), 9)) + "  " + sp("g", pad("✓ cleaned", 11)) + " " + sp("s", pad(it.name, nW));
      } else {
        ln += sp("b", padL(fmt(it.size), 9)) + "  " + badge(it.level) + " " + sp("b", pad(it.name, nW)) + "  " + sp("f", trunc(it.how, rest - nW - 2));
      }
      out += `<span class="row ln${on ? " on" : ""}" data-item="${i}">${ln}</span>`;
    });
    const it = items[st.icur];
    const action = it.kind === "trash" ? "moves the folder to the Trash" : it.kind === "cmd" ? "runs " + it.cmd : it.kind === "family" ? "hands it back to " + it.tool : "none: " + it.how;
    const pane = `<span class="ln">${sp("f", "─".repeat(cols))}</span>` +
      `<span class="ln">${sp("b", trunc(it.name, cols))}  ${sp("f", trunc(it.path + "  " + fmt(it.size0) + " on disk", Math.max(0, cols - it.name.length - 2)))}</span>` +
      `<span class="ln">${esc(trunc(it.what, cols))}</span>` +
      `<span class="ln">${trunc("Action: ", cols)}${sp("m", trunc(action, cols - 8))}${sp("f", trunc("  ·  comes back with: ", Math.max(0, cols - 8 - action.length)))}${esc(trunc(it.back, Math.max(0, cols - 30 - action.length)))}</span>` +
      `<span class="ln f">${esc(trunc("Why " + it.level + ": " + it.why, cols))}</span>`;
    return { body: out, foot: pane + status("↑↓ move  space select  s safe  a all  enter clean  t treemap  ← back  ? help") };
  };

  // Squarified treemap (Bruls, Huizing, van Wijk), on a w×h rectangle.
  const squarify = (vals, x, y, w, h) => {
    const total = sum(vals, (v) => v.v);
    const area = vals.map((v) => ({ ...v, a: (v.v / total) * w * h }));
    const out = [];
    const worst = (row, side) => {
      const s = sum(row, (r) => r.a), mx = Math.max(...row.map((r) => r.a)), mn = Math.min(...row.map((r) => r.a));
      return Math.max((side * side * mx) / (s * s), (s * s) / (side * side * mn));
    };
    let row = [], rest = area.slice();
    const layout = (row) => {
      const s = sum(row, (r) => r.a);
      if (w >= h) {
        const rw = s / h; let yy = y;
        row.forEach((r) => { const rh = r.a / rw; out.push({ ...r, x, y: yy, w: rw, h: rh }); yy += rh; });
        x += rw; w -= rw;
      } else {
        const rh = s / w; let xx = x;
        row.forEach((r) => { const rw = r.a / rh; out.push({ ...r, x: xx, y, w: rw, h: rh }); xx += rw; });
        y += rh; h -= rh;
      }
    };
    while (rest.length) {
      const side = Math.min(w, h), next = rest[0];
      if (!row.length || worst(row, side) >= worst([...row, next], side)) { row.push(next); rest.shift(); }
      else { layout(row); row = []; }
    }
    if (row.length) layout(row);
    return out;
  };

  const treemap = () => {
    const cats = visibleCats();
    let vals;
    if (st.zoom == null) {
      vals = cats.map((c, i) => ({ v: sum(c.items, (x) => x.size0), c, i, label: c.name, color: c.color,
        sub: left(c).length ? fmt(catSize(c)) + " · " + left(c).length + (left(c).length === 1 ? " item" : " items") : "cleaned", done: !left(c).length,
        sel: left(c).length && left(c).filter(selectable).every((x) => x.sel) && left(c).some((x) => x.sel) }));
    } else {
      const c = cats[st.zoom];
      vals = c.items.map((it, i) => ({ v: it.size0, it, i, label: it.name, color: c.color, sub: fmt(it.size0) + " · " + icon[it.level] + " " + it.level, done: it.done, sel: it.sel }));
    }
    vals.sort((a, b) => b.v - a.v);
    st.tcur = Math.min(st.tcur, vals.length - 1);
    const tiles = squarify(vals, 0, 0, 100, 100);
    const title = st.zoom == null ? "All categories" : cats[st.zoom].name;
    let out = `<span class="ln">${sp("m b", title)}  ${sp("f", st.zoom == null ? "·  enter zooms into a category" : "·  esc zooms out")}</span>`;
    out += '<div class="tm">';
    tiles.forEach((t, k) => {
      const col = t.color;
      const light = (() => { const n = parseInt(col.slice(1), 16); const r = n >> 16, g = (n >> 8) & 255, b = n & 255; return (0.299 * r + 0.587 * g + 0.114 * b) / 255 > 0.55; })();
      const shadeV = st.zoom == null ? 0 : (k % 4) * 0.09;
      out += `<div class="tile${t.done ? " done" : ""}${t.sel ? " sel" : ""}${k === st.tcur ? " on" : ""}" data-tile="${k}" data-idx="${t.i}" style="left:${t.x}%;top:${t.y}%;width:${t.w}%;height:${t.h}%;background:${col};filter:brightness(${1 - shadeV});color:${light ? "#14161a" : "#f4f5f7"}"><b>${esc(t.label)}</b>${esc(t.sub)}</div>`;
    });
    out += "</div>";
    st._tiles = tiles;
    return { body: out, foot: status("←↑↓→ move  enter zoom in  esc zoom out  space select  c clean  t list  ? help") };
  };

  const plan = () => {
    const sel = selected();
    const groups = { trash: sel.filter((i) => i.kind === "trash"), cmd: sel.filter((i) => i.kind === "cmd"), family: sel.filter((i) => i.kind === "family") };
    let out = `<span class="ln">${sp("m b", "Clean " + sel.length + " item(s)")}  ${sp("f", "·")}  ${sp("b", "frees " + fmt(sum(sel, (i) => i.size)))}</span>` + lines(1);
    const block = (title, cls, arr, how) => {
      if (!arr.length) return;
      out += `<span class="ln">${sp(cls + " b", title)}  ${sp("f", fmt(sum(arr, (i) => i.size)) + " · " + arr.length + " item(s) · " + how)}</span>`;
      arr.slice(0, 4).forEach((i) => { out += `<span class="ln">   ${sw(i.cat.color)}${esc(pad(i.name, 40))} ${sp("f", trunc(i.kind === "cmd" ? i.cmd : i.how, cols - 48))}</span>`; });
      if (arr.length > 4) out += `<span class="ln f">   … ${arr.length - 4} more</span>`;
    };
    block("Move to the Trash", "g", groups.trash, "Put Back restores");
    block("Run official commands", "y", groups.cmd, "they delete by their own design");
    block("Hand back to family tools", "c", groups.family, "each re-checks with its own rules");
    out += lines(1) + `<span class="ln f">${esc(trunc("Every item is re-read and fingerprint-checked right before it is touched.", cols))}</span>`;
    return { body: out, foot: `<span class="ln">${sp("b", "y")} ${sp("f", "run")}   ${sp("b", "esc")} ${sp("f", "cancel")}</span>` };
  };

  const spin = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏";
  const running = () => {
    let out = `<span class="ln">${sp("m b", "Cleaning")}  ${sp("f", "·  ctrl+c stops after the running step")}</span>` + lines(1);
    const shown = st.steps.slice(Math.max(0, st.steps.length - (rows - 6)));
    shown.forEach((s) => {
      const m = s.done ? sp("g", "✓") : sp("m", spin[Math.floor(performance.now() / 80) % spin.length]);
      out += `<span class="ln"> ${m} ${esc(pad(s.label, Math.min(46, cols - 30)))} ${sp("f", trunc(s.how, Math.max(0, cols - 60)))}</span>`;
    });
    return { body: out, foot: "" };
  };

  const receipt = () => {
    const r = st.receipt;
    const nb = Math.max(10, Math.min(60, cols - 30));
    let out = `<span class="ln">${sp("m b", "Done in " + r.secs + "s")}</span>` + lines(1);
    out += `<div class="rbars" style="grid-template-columns:7ch ${nb}ch 14ch">` +
      `<span>Before</span><span class="track"><i style="width:${(FREE0 / DISK) * 100}%"></i></span><span>&nbsp;${esc(fmt(FREE0))} free</span>` +
      `<span>After</span><span class="track"><i data-spring="after" style="width:${spring("after", FREE0 / DISK).x * 100}%"></i></span><span class="b">&nbsp;${esc(fmt(st.free))} free</span></div>`;
    out += lines(1);
    const line = (label, arr, desc) => arr.length ? `<span class="ln">${sp("g", "✓ " + pad(label, 18))}${padL(fmt(sum(arr, (i) => i.size0)), 10)}  ${padL(String(arr.length), 4)} item(s)  ${sp("g", trunc(desc, Math.max(0, cols - 48)))}</span>` : "";
    const cmds = [...new Set(r.cmd.map((i) => i.cmd.split(" ").slice(0, i.cmd.startsWith("xcrun simctl delete 9") ? 3 : 4).join(" ").replace(/ [0-9A-F]{8}-.*$/, "")))];
    const fam = Object.entries(r.family.reduce((m, i) => (m[i.tool] = (m[i.tool] || 0) + 1, m), {})).map(([t, n]) => t + " ×" + n).join(" · ");
    out += line("Moved to Trash", r.trash, "freed when you empty the Trash; Put Back restores");
    out += line("Freed now", r.cmd, cmds.slice(0, 3).join(" · ") + (cmds.length > 3 ? " · " + (cmds.length - 3) + " more" : ""));
    out += line("Delegated", r.family, fam + " (each tool's own rules)");
    const trashMore = sum(r.trash, (i) => i.size0);
    out += `<span class="ln">  ${sp("b", trunc("Free space  " + fmt(FREE0) + " → " + fmt(st.free) + "   +" + fmt(st.free - FREE0) + " measured" + (trashMore ? " · about " + fmt(trashMore) + " more when you empty the Trash" : ""), cols - 2))}</span>`;
    out += `<span class="ln f">  ${esc(trunc("↺ Build caches (Go, Homebrew, DerivedData) come back as you work; that is normal.", cols - 2))}</span>`;
    return { body: out, foot: `<span class="ln f">any key returns to the overview  ·  R reset the demo</span>` };
  };

  const help = () => {
    const out = [
      sp("m b", "Safety levels"),
      `  ${sp("g", "● safe")}       regrows or is unusable; ${sp("b", "s")} and --yes take only these`,
      `  ${sp("y", "▲ review")}     yours to judge; never taken by s or --yes`,
      `  ${sp("r", "■ protected")}  shown with the owner's way to remove it, never acted on`,
      `  ${sp("m", "◉ live")}       in use right now (Xcode running, simulator booted), not selectable`,
      "",
      sp("m b", "Keys"),
      "  ↑↓ / jk  move          enter  open / zoom     esc  back",
      "  space    select        s      select safe     c    clean (shows the plan)",
      "  t        treemap       R      reset demo      ?    this help",
      "",
      sp("f", "The real TUI also explains every colour and column here."),
    ].map((l) => `<span class="ln">${l || " "}</span>`).join("");
    return { body: out, foot: `<span class="ln f">any key closes help</span>` };
  };

  /* ---------- render ---------- */
  const render = () => {
    measure();
    let v;
    if (st.help) v = help();
    else v = ({ overview, list, treemap, plan, running, receipt })[st.view]();
    term.innerHTML = `<div class="screen">${header()}${v.body}</div><div class="tfoot">${v.foot}</div>`;
    paintSprings();
    renderKeys();
    animate();
  };

  const KEYSETS = {
    overview: [["s", "select safe", true], ["c", "clean"], ["enter", "open"], ["t", "treemap"], ["space", "select"], ["R", "reset"], ["?", "help"]],
    list: [["space", "select"], ["a", "all"], ["s", "safe"], ["c", "clean"], ["esc", "back"]],
    treemap: [["enter", "zoom in"], ["esc", "zoom out"], ["space", "select"], ["s", "select safe"], ["c", "clean"], ["t", "list"]],
    plan: [["y", "run", true], ["esc", "cancel"]],
    running: [],
    receipt: [["x", "back to overview", true], ["R", "reset"]],
  };
  const renderKeys = () => {
    let set = st.help ? [["esc", "close help"]] : KEYSETS[st.view];
    if (st.view === "overview" && selected().length) set = set.map((k) => [k[0], k[1], k[0] === "c"]);
    keysEl.innerHTML = [["↑", ""], ["↓", ""]].concat(st.view === "treemap" ? [["←", ""], ["→", ""]] : []).filter(() => st.view !== "plan" && st.view !== "receipt" && st.view !== "running" && !st.help)
      .concat(set).map(([k, l, p]) => `<button type="button" data-key="${esc(k)}" class="${p ? "primary pulse" : ""}"><b>${esc(k)}</b>${esc(l)}</button>`).join("");
  };

  /* ---------- actions ---------- */
  const selectSafe = (items) => items.filter((i) => !i.done && i.level === "safe").forEach((i) => (i.sel = true));
  const toggle = (items) => {
    const s = items.filter(selectable);
    const all = s.length && s.every((i) => i.sel);
    s.forEach((i) => (i.sel = !all));
  };

  const run = () => {
    const sel = selected();
    if (!sel.length) return;
    st.view = "running";
    st.steps = [];
    const groups = [
      ...sel.filter((i) => i.kind === "trash").map((i) => ({ label: "Trash · " + i.name, how: "→ Trash", items: [i] })),
      ...[...new Set(sel.filter((i) => i.kind === "cmd").map((i) => i.cmd))].map((cmd) => ({ label: cmd, how: "", items: sel.filter((i) => i.cmd === cmd) })),
      ...[...new Set(sel.filter((i) => i.kind === "family").map((i) => i.tool))].map((t) => ({ label: t + " apply --plan -", how: "{id, fingerprint} × " + sel.filter((i) => i.tool === t).length, items: sel.filter((i) => i.tool === t) })),
    ];
    const t0 = performance.now();
    let k = 0;
    const tick = () => {
      if (st.view !== "running") return;
      if (k > 0) st.steps[k - 1].done = true;
      if (k < groups.length) { st.steps.push({ ...groups[k], done: false }); k++; render(); setTimeout(tick, reduceMotion ? 40 : 110 + Math.random() * 120); }
      else finish();
    };
    const spinLoop = () => { if (st.view === "running") { render(); setTimeout(spinLoop, 90); } };
    const finish = () => {
      sel.forEach((i) => { i.done = true; i.sel = false; i.size = 0; });
      const freedNow = sum(sel.filter((i) => i.kind === "cmd"), (i) => i.size0);
      st.free += freedNow;
      st.cleaned = true;
      st.receipt = { secs: Math.max(1, Math.round((performance.now() - t0) / 1000)), trash: sel.filter((i) => i.kind === "trash"), cmd: sel.filter((i) => i.kind === "cmd"), family: sel.filter((i) => i.kind === "family") };
      st.view = "receipt";
      render();
      spring("after", st.free / DISK);
      animate();
    };
    tick(); spinLoop();
  };

  const reset = () => {
    ALL.forEach((i) => { i.done = false; i.sel = false; i.size = i.size0; });
    Object.assign(st, { view: "overview", cur: 0, icur: 0, tcur: 0, zoom: null, free: FREE0, cleaned: false, steps: [], receipt: null, help: false });
    springs.delete("after");
    render(); animate();
  };

  const key = (k) => {
    if (hint) hint.classList.add("gone");
    if (st.help) { st.help = false; render(); return; }
    const cats = visibleCats();
    if (k === "R") return reset();
    if (k === "?") { st.help = true; return render(); }
    switch (st.view) {
      case "overview":
        if (k === "up") st.cur = Math.max(0, st.cur - 1);
        else if (k === "down") st.cur = Math.min(cats.length - 1, st.cur + 1);
        else if (k === "enter" || k === "right") { st.view = "list"; st.icur = 0; }
        else if (k === "space") toggle(cats[st.cur].items);
        else if (k === "s") selectSafe(ALL);
        else if (k === "c") { if (!selected().length) selectSafe(ALL); st.view = "plan"; }
        else if (k === "t") { st.view = "treemap"; st.zoom = null; st.tcur = 0; }
        else return false;
        break;
      case "list": {
        const items = cats[st.cur].items;
        if (k === "up") st.icur = Math.max(0, st.icur - 1);
        else if (k === "down") st.icur = Math.min(items.length - 1, st.icur + 1);
        else if (k === "esc" || k === "left") st.view = "overview";
        else if (k === "space") { const it = items[st.icur]; if (selectable(it)) it.sel = !it.sel; }
        else if (k === "a") toggle(items);
        else if (k === "s") selectSafe(items);
        else if (k === "c" || k === "enter") { if (!selected().length) { const it = items[st.icur]; if (selectable(it)) it.sel = true; else return render(); } st.view = "plan"; }
        else if (k === "t") { st.view = "treemap"; st.zoom = st.cur; st.tcur = 0; }
        else return false;
        break;
      }
      case "treemap": {
        const tiles = st._tiles || [];
        const t = tiles[st.tcur];
        if (["up", "down", "left", "right"].includes(k) && t) {
          // nearest tile in that direction, by centre distance
          const cx = t.x + t.w / 2, cy = t.y + t.h / 2;
          let best = -1, bd = Infinity;
          tiles.forEach((o, i) => {
            if (i === st.tcur) return;
            const ox = o.x + o.w / 2, oy = o.y + o.h / 2, dx = ox - cx, dy = oy - cy;
            const ok = k === "left" ? o.x + o.w <= t.x + 0.01 : k === "right" ? o.x >= t.x + t.w - 0.01 : k === "up" ? o.y + o.h <= t.y + 0.01 : o.y >= t.y + t.h - 0.01;
            if (!ok) return;
            const d = (k === "left" || k === "right") ? Math.abs(dx) + 2 * Math.abs(dy) : Math.abs(dy) + 2 * Math.abs(dx);
            if (d < bd) { bd = d; best = i; }
          });
          if (best >= 0) st.tcur = best;
        } else if (k === "enter") { if (st.zoom == null && t) { st.zoom = t.i; st.cur = t.i; st.tcur = 0; } }
        else if (k === "esc") { if (st.zoom != null) { st.tcur = 0; st.zoom = null; } else st.view = "overview"; }
        else if (k === "space" && t) { if (st.zoom == null) toggle(cats[t.i].items); else { const it = cats[st.zoom].items[t.i]; if (selectable(it)) it.sel = !it.sel; } }
        else if (k === "s") selectSafe(st.zoom == null ? ALL : cats[st.zoom].items);
        else if (k === "c") { if (!selected().length) selectSafe(ALL); st.view = "plan"; }
        else if (k === "t") st.view = st.zoom == null ? "overview" : "list";
        else return false;
        break;
      }
      case "plan":
        if (k === "y" || k === "enter") return run();
        if (k === "esc" || k === "n" || k === "q") st.view = "overview";
        else return false;
        break;
      case "running": return true;
      case "receipt": st.view = "overview"; st.cur = 0; break;
    }
    render();
    return true;
  };

  const KEYMAP = { ArrowUp: "up", k: "up", ArrowDown: "down", j: "down", ArrowLeft: "left", h: "left", ArrowRight: "right", l: "right", Enter: "enter", " ": "space", Escape: "esc", Backspace: "esc" };
  term.addEventListener("keydown", (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const k = KEYMAP[e.key] || (e.key.length === 1 ? e.key : null);
    if (!k) return;
    const handled = key(k === "x" && st.view !== "receipt" ? "esc" : k);
    if (handled !== false) e.preventDefault();
  });
  keysEl.addEventListener("click", (e) => {
    const b = e.target.closest("button[data-key]");
    if (!b) return;
    const map = { "↑": "up", "↓": "down", "←": "left", "→": "right" };
    const k = map[b.dataset.key] || b.dataset.key;
    key(k === "x" ? "x" : k);
  });
  term.addEventListener("click", (e) => {
    term.focus({ preventScroll: true });
    const row = e.target.closest("[data-cat],[data-item],[data-tile]");
    if (!row) { if (st.view === "receipt" || st.help) key("x"); return; }
    if (row.dataset.cat != null) { const i = +row.dataset.cat; if (i === st.cur) key("enter"); else { st.cur = i; render(); } }
    else if (row.dataset.item != null) { const i = +row.dataset.item; if (i === st.icur) key("space"); else { st.icur = i; render(); } }
    else if (row.dataset.tile != null) { const i = +row.dataset.tile; if (i === st.tcur) key("enter"); else { st.tcur = i; render(); } }
  });

  let rt;
  addEventListener("resize", () => { clearTimeout(rt); rt = setTimeout(render, 80); });

  // First paint: bars spring in from nothing, the way a fresh scan lands.
  CATS.forEach((c) => springs.set("bar:" + c.name, { x: 0, v: 0, t: 0 }));
  render();
  if ("IntersectionObserver" in window && !reduceMotion) {
    const io = new IntersectionObserver((es) => { if (es[0].isIntersecting) { render(); animate(); io.disconnect(); } }, { threshold: 0.3 });
    io.observe(term);
  } else { render(); animate(); }
})();
