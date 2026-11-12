"use strict";

const fmtInt = new Intl.NumberFormat("es-ES");
const fmtNum = new Intl.NumberFormat("es-ES", { maximumFractionDigits: 1 });
const fmtPct = (v) => `${(v * 100).toFixed(2)}%`;

function node(tag, cls, text) {
  const el = document.createElement(tag);
  if (cls) el.className = cls;
  if (text !== undefined && text !== null) el.textContent = text;
  return el;
}

function ms(v) {
  if (!v || v <= 0) return "0 ms";
  if (v >= 1000) return `${(v / 1000).toFixed(2)} s`;
  if (v >= 100) return `${Math.round(v)} ms`;
  return `${v.toFixed(1)} ms`;
}

function bytes(v) {
  if (!v) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(Math.floor(Math.log(v) / Math.log(1024)), units.length - 1);
  const n = v / Math.pow(1024, i);
  return `${n.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatDate(iso) {
  try {
    return new Date(iso).toLocaleString("es-ES", { dateStyle: "medium", timeStyle: "short" });
  } catch (_) {
    return iso || "—";
  }
}

function codeClass(code) {
  if (code >= 200 && code < 300) return "ok";
  if (code >= 300 && code < 400) return "info";
  if (code >= 400 && code < 500) return "warn";
  if (code >= 500) return "bad";
  return "";
}

function renderVerdict(r) {
  const v = r.verdict || {};
  const box = document.getElementById("verdict");
  box.classList.remove("pass", "limit", "fail", "interrupted");
  box.classList.add(v.status || "fail");

  const icons = { pass: "✓", limit: "▲", fail: "✕", interrupted: "◼" };
  document.getElementById("vIcon").textContent = icons[v.status] || "•";
  document.getElementById("vTitle").textContent = v.title || "Resultado";
  document.getElementById("vMsg").textContent = v.message || "";

  const rateBox = document.getElementById("vRateBox");
  if (v.status === "pass" || v.status === "limit") {
    let rate = v.breaking_rate;
    if (v.status === "pass") {
      rate = r.meta.mode === "ramp" ? r.meta.rate_max : (r.meta.rate_max || r.meta.rate_start);
    }
    if (!rate) rate = r.meta.rate_max;
    document.getElementById("vRate").textContent = fmtNum.format(rate);
    document.getElementById("vRateLabel").textContent =
      v.status === "limit" ? "req/s en el límite" : "req/s sostenidos";
    rateBox.style.display = "";
  } else {
    rateBox.style.display = "none";
  }
}

function renderCards(r) {
  const s = r.summary, l = r.latency, t = r.thresholds;
  let p95Class = "";
  if (t.max_p95_ms > 0) {
    if (l.p95 <= t.max_p95_ms) p95Class = "good";
    else if (l.p95 <= t.max_p95_ms * 1.5) p95Class = "warn";
    else p95Class = "bad";
  }
  const successClass = s.success_rate >= 0.99 ? "good" : s.success_rate >= 0.95 ? "warn" : "bad";
  const cards = [
    { label: "Peticiones", value: fmtInt.format(s.requests), sub: `${fmtNum.format(s.rate)} req/s de media` },
    { label: "Tasa de éxito", value: fmtPct(s.success_rate), sub: `${fmtPct(s.error_rate)} errores`, cls: successClass },
    { label: "Latencia P95", value: ms(l.p95), sub: t.max_p95_ms > 0 ? `umbral ${ms(t.max_p95_ms)}` : "sin umbral", cls: p95Class },
    { label: "Latencia P99", value: ms(l.p99), sub: `máx ${ms(l.max)}` },
    { label: "Throughput", value: `${fmtNum.format(s.throughput)} req/s`, sub: "peticiones exitosas" },
    { label: "Datos recibidos", value: bytes(s.bytes_in), sub: `${bytes(s.bytes_out)} enviados` },
  ];

  const wrap = document.getElementById("cards");
  wrap.replaceChildren();
  for (const c of cards) {
    const card = node("div", "card" + (c.cls ? " " + c.cls : ""));
    card.append(node("div", "label", c.label), node("div", "value", c.value), node("div", "sub", c.sub));
    wrap.append(card);
  }
}

function chartBase(el, series, yValues) {
  return {
    width: Math.max(el.clientWidth || 560, 280),
    height: 260,
    padding: [12, 12, 0, 0],
    cursor: { drag: { x: true, y: false } },
    scales: { x: { time: false } },
    axes: [
      {
        stroke: "#64748b",
        grid: { stroke: "#13203a", width: 1 },
        ticks: { stroke: "#1e293b", width: 1 },
        font: "10px ui-sans-serif, system-ui, sans-serif",
        size: 36,
      },
      {
        stroke: "#64748b",
        grid: { stroke: "#13203a", width: 1 },
        ticks: { stroke: "#1e293b", width: 1 },
        font: "10px ui-sans-serif, system-ui, sans-serif",
        size: 54,
        values: (_u, splits) => splits.map(yValues),
      },
    ],
    series,
    legend: { show: true, live: false },
  };
}

function renderCharts(r) {
  const timeline = r.timeline || [];
  const latEl = document.getElementById("latencyChart");
  const rpsEl = document.getElementById("rpsChart");

  if (!timeline.length) {
    latEl.append(node("div", "empty", "Sin datos de latencia para este periodo."));
    rpsEl.append(node("div", "empty", "Sin datos de carga para este periodo."));
    return;
  }

  const xs = timeline.map((p) => p.t);
  const latData = [
    xs,
    timeline.map((p) => p.p50),
    timeline.map((p) => p.p95),
    timeline.map((p) => p.p99),
  ];
  const rpsData = [
    xs,
    timeline.map((p) => p.target_rate),
    timeline.map((p) => p.rate),
  ];

  const latOpts = chartBase(
    latEl,
    [
      {},
      { label: "P50", stroke: "#22d3ee", width: 2 },
      { label: "P95", stroke: "#fbbf24", width: 2 },
      { label: "P99", stroke: "#fb7185", width: 2 },
    ],
    (v) => (v >= 1000 ? `${(v / 1000).toFixed(1)}s` : `${Math.round(v)}ms`)
  );
  const rpsOpts = chartBase(
    rpsEl,
    [
      {},
      { label: "objetivo", stroke: "#64748b", width: 1.5 },
      { label: "real", stroke: "#34d399", width: 2 },
    ],
    (v) => `${Math.round(v)}`
  );

  const charts = [new uPlot(latOpts, latData, latEl), new uPlot(rpsOpts, rpsData, rpsEl)];
  const resize = () => {
    for (const c of charts) c.setSize({ width: Math.max(c.root.clientWidth, 280), height: 260 });
  };
  if (window.ResizeObserver) {
    const ro = new ResizeObserver(resize);
    ro.observe(latEl);
    ro.observe(rpsEl);
  }
  window.addEventListener("resize", resize);
}

function renderDist(r) {
  const wrap = document.getElementById("dist");
  wrap.replaceChildren();
  const items = r.latency_distribution || [];
  const max = Math.max(...items.map((b) => b.count), 1);
  if (!items.some((b) => b.count > 0)) {
    wrap.append(node("div", "empty", "Sin respuestas registradas."));
    return;
  }
  for (const b of items) {
    const row = node("div", "bar-row");
    const track = node("div", "track");
    const fill = node("div", "fill");
    fill.style.width = `${(b.count / max) * 100}%`;
    track.append(fill);
    row.append(
      node("span", "bar-label", b.label),
      track,
      node("span", "num", fmtInt.format(b.count)),
      node("span", "pct", `${((b.count / r.summary.requests) * 100 || 0).toFixed(1)}%`)
    );
    wrap.append(row);
  }
}

function renderCodes(r) {
  const wrap = document.getElementById("codes");
  wrap.replaceChildren();
  const items = r.status_codes || [];
  if (!items.length) {
    wrap.append(node("div", "empty", "Sin respuestas HTTP."));
    return;
  }
  const max = Math.max(...items.map((s) => s.count), 1);
  for (const s of items) {
    const cls = codeClass(s.code);
    const row = node("div", "bar-row");
    const badge = node("span", "code-badge " + cls, `${s.code}`);
    badge.style.width = "52px";
    const track = node("div", "track");
    const fill = node("div", "fill " + cls);
    fill.style.width = `${(s.count / max) * 100}%`;
    track.append(fill);
    row.append(badge, track, node("span", "num", fmtInt.format(s.count)), node("span", "pct", fmtPct(s.percent)));
    wrap.append(row);
  }
}

function renderErrors(r) {
  const wrap = document.getElementById("errors");
  wrap.replaceChildren();
  const items = r.errors || [];
  if (!items.length) {
    wrap.append(node("div", "empty good", "Sin errores. Todo limpio. ✓"));
    return;
  }
  for (const e of items) {
    const row = node("div", "error-row");
    const msg = node("span", "error-msg", e.message);
    msg.title = e.message;
    row.append(node("span", "error-count", `× ${fmtInt.format(e.count)}`), msg);
    wrap.append(row);
  }
}

function renderRecommendations(r) {
  const wrap = document.getElementById("recs");
  wrap.replaceChildren();
  const items = (r.verdict && r.verdict.recommendations) || [];
  if (!items.length) {
    const li = node("li", "good", "Todo dentro de los umbrales configurados. No hay recomendaciones urgentes.");
    wrap.append(li);
    return;
  }
  for (const text of items) wrap.append(node("li", "", text));
}

function renderMeta(r) {
  const m = r.meta, e = r.environment;
  const grid = document.getElementById("metaGrid");
  grid.replaceChildren();

  const mode = m.mode === "ramp"
    ? `ramp · ${fmtNum.format(m.rate_start)} → ${fmtNum.format(m.rate_max)} req/s`
    : `constant · ${fmtNum.format(m.rate_max || m.rate_start)} req/s`;

  const items = [
    { k: "Objetivo", v: m.target, wide: true },
    { k: "Método", v: m.method },
    { k: "Modo", v: mode },
    {
      k: "Duración",
      v: `${(m.duration_ms / 1000).toFixed(1)} s reales · ${(m.planned_duration_ms / 1000).toFixed(1)} s planificados`,
    },
    { k: "Concurrencia", v: `${m.workers} workers iniciales · ${m.max_workers} máximo` },
    { k: "Timeout", v: m.timeout_ms > 0 ? ms(m.timeout_ms) : "sin timeout" },
    { k: "TLS", v: m.insecure ? "verificación desactivada (--insecure)" : "verificado" },
    { k: "Inicio", v: formatDate(m.started_at) },
    { k: "Entorno", v: `${e.os}/${e.arch} · ${e.cpus} CPUs · ${e.go_version}` },
    { k: "Versión", v: `${m.tool} ${m.version}` },
  ];
  if (m.body_bytes > 0) items.push({ k: "Cuerpo enviado", v: `${fmtInt.format(m.body_bytes)} bytes` });
  if (m.auto_stopped && m.stop_reason) items.push({ k: "Detención automática", v: m.stop_reason, wide: true });
  if (m.interrupted) items.push({ k: "Estado", v: "Prueba interrumpida por el usuario" });

  for (const item of items) {
    const box = node("div", "meta-item" + (item.wide ? " wide" : ""));
    box.append(node("div", "k", item.k), node("div", "v", item.v));
    grid.append(box);
  }

  const chips = document.getElementById("headerChips");
  chips.replaceChildren();
  if (m.header_names && m.header_names.length) {
    chips.append(node("span", "chip-label", "Headers (solo nombres):"));
    for (const h of m.header_names) chips.append(node("span", "chip", h));
  }
  if (m.body_bytes > 0) chips.append(node("span", "chip", `body · ${fmtInt.format(m.body_bytes)} B`));

  document.getElementById("metaWhen").textContent = formatDate(m.started_at);
  document.getElementById("version").textContent = m.version;
  document.getElementById("env").textContent = `${e.os}/${e.arch} · ${e.cpus} CPUs`;
}

function render(r) {
  document.title = `LoadForge · ${r.verdict.title}`;
  renderVerdict(r);
  renderCards(r);
  renderCharts(r);
  renderDist(r);
  renderCodes(r);
  renderErrors(r);
  renderRecommendations(r);
  renderMeta(r);
}

(async function main() {
  try {
    const res = await fetch("/api/report", { cache: "no-store" });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    render(await res.json());
  } catch (err) {
    const box = document.getElementById("verdict");
    box.classList.add("fail");
    document.getElementById("vIcon").textContent = "✕";
    document.getElementById("vTitle").textContent = "No se pudo cargar el reporte";
    document.getElementById("vMsg").textContent = String(err.message || err);
    document.getElementById("vRateBox").style.display = "none";
  }
})();
