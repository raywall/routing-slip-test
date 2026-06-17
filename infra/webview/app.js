const STORAGE_KEY = "custom-business-metrics.webview.settings.v2";
const DASHBOARD_KEY = "custom-business-metrics.webview.dashboard.v1";

const WIDGET_TYPES = [
  { type: "bar", label: "Bar chart", icon: "▥", query: "count:processes{status:completed}.rollup(count, 60)", w: 4, h: 2 },
  { type: "pie", label: "Pie chart", icon: "◔", query: "count:processes{group_by:status}", w: 3, h: 2 },
  { type: "point", label: "Point plot", icon: "⠿", query: "point:duration_ms{*}", w: 4, h: 2 },
  { type: "query_value", label: "Query value", icon: "#", query: "count:processes{*}", w: 3, h: 1 },
  { type: "table", label: "Table", icon: "▤", query: "table:processes{*}", w: 6, h: 3 },
  { type: "timeseries", label: "Timeseries", icon: "⌁", query: "count:processes{*}.rollup(count, 60)", w: 5, h: 2 },
  { type: "top_list", label: "Top list", icon: "≡", query: "top:workflow{*}", w: 4, h: 2 },
];

const state = {
  settings: {
    apiUrl: "http://localhost:8080",
    apiKey: "",
    rangeMode: "15m",
    rangeFrom: "",
    rangeTo: "",
    refreshInterval: 5000,
    showCustomMetrics: false,
    theme: "dark",
  },
  timer: null,
  events: [],
  processes: [],
  previousProcesses: [],
  chartProcesses: [],
  editMode: false,
  widgets: [],
  editingWidgetId: null,
  draggedWidgetId: null,
  resizing: null,
};

const els = {
  apiUrl: document.querySelector("#api-url"),
  apiKey: document.querySelector("#api-key"),
  rangeMode: document.querySelector("#range-mode"),
  customRange: document.querySelector("#custom-range"),
  rangeFrom: document.querySelector("#range-from"),
  rangeTo: document.querySelector("#range-to"),
  refreshInterval: document.querySelector("#refresh-interval"),
  showCustomMetrics: document.querySelector("#show-custom-metrics"),
  refreshNow: document.querySelector("#refresh-now"),
  themeToggle: document.querySelector("#theme-toggle"),
  themeToggleIcon: document.querySelector("#theme-toggle-icon"),
  openConfig: document.querySelector("#open-config"),
  saveConfig: document.querySelector("#save-config"),
  configModal: document.querySelector("#config-modal"),
  dot: document.querySelector("#status-dot"),
  statusText: document.querySelector("#status-text"),
  lastUpdate: document.querySelector("#last-update"),
  chartSubtitle: document.querySelector("#chart-subtitle"),
  hourlyChart: document.querySelector("#hourly-chart"),
  processSummary: document.querySelector("#process-summary"),
  processFilter: document.querySelector("#process-filter"),
  processSearch: document.querySelector("#process-search"),
  processList: document.querySelector("#process-list"),
  kpiTotal: document.querySelector("#kpi-total"),
  kpiSuccess: document.querySelector("#kpi-success"),
  kpiErrors: document.querySelector("#kpi-errors"),
  kpiReprocess: document.querySelector("#kpi-reprocess"),
  kpiTotalSub: document.querySelector("#kpi-total-sub"),
  kpiSuccessSub: document.querySelector("#kpi-success-sub"),
  kpiErrorsSub: document.querySelector("#kpi-errors-sub"),
  kpiReprocessSub: document.querySelector("#kpi-reprocess-sub"),
  chartPeak: document.querySelector("#chart-peak"),
  workflowTopList: document.querySelector("#workflow-top-list"),
  processModal: document.querySelector("#process-modal"),
  modalClose: document.querySelector("#modal-close"),
  modalTitle: document.querySelector("#modal-title"),
  modalCopy: document.querySelector("#modal-copy"),
  modalBody: document.querySelector("#modal-body"),
  modalStatusPill: document.querySelector("#modal-status-pill"),
  editDashboardToggle: document.querySelector("#edit-dashboard-toggle"),
  dashboardEditHint: document.querySelector("#dashboard-edit-hint"),
  metricsGrid: document.querySelector("#metrics-grid"),
  widgetPalette: document.querySelector("#widget-palette"),
  widgetPaletteList: document.querySelector("#widget-palette-list"),
  widgetEditorModal: document.querySelector("#widget-editor-modal"),
  widgetEditorClose: document.querySelector("#widget-editor-close"),
  widgetEditorTitle: document.querySelector("#widget-editor-title"),
  widgetTitleInput: document.querySelector("#widget-title-input"),
  widgetTypeInput: document.querySelector("#widget-type-input"),
  widgetQueryInput: document.querySelector("#widget-query-input"),
  widgetPreviewRefresh: document.querySelector("#widget-preview-refresh"),
  widgetPreviewBody: document.querySelector("#widget-preview-body"),
  widgetSave: document.querySelector("#widget-save"),
};

function loadSettings() {
  try {
    state.settings = { ...state.settings, ...JSON.parse(localStorage.getItem(STORAGE_KEY) || "{}") };
  } catch {
    localStorage.removeItem(STORAGE_KEY);
  }
  els.apiUrl.value = state.settings.apiUrl;
  els.apiKey.value = state.settings.apiKey;
  els.rangeMode.value = state.settings.rangeMode;
  els.rangeFrom.value = state.settings.rangeFrom;
  els.rangeTo.value = state.settings.rangeTo;
  els.refreshInterval.value = String(state.settings.refreshInterval);
  els.showCustomMetrics.checked = Boolean(state.settings.showCustomMetrics);
  syncRangeControls();
  syncCustomMetricsVisibility();
  applyTheme(state.settings.theme);
}

function loadDashboard() {
  try {
    const stored = JSON.parse(localStorage.getItem(DASHBOARD_KEY) || "{}");
    state.widgets = Array.isArray(stored.widgets) ? stored.widgets : defaultWidgets();
  } catch {
    state.widgets = defaultWidgets();
  }
  if (state.widgets.length === 0) state.widgets = defaultWidgets();
}

function defaultWidgets() {
  return [
    newWidget("query_value", { title: "Processamentos", query: "count:processes{*}", w: 3, h: 1 }),
    newWidget("query_value", { title: "Sucesso", query: "count:processes{status:completed}", w: 3, h: 1 }),
    newWidget("query_value", { title: "Erros", query: "count:processes{status:failed}", w: 3, h: 1 }),
    newWidget("query_value", { title: "Reprocessamentos", query: "count:reprocesses{*}", w: 3, h: 1 }),
  ];
}

function saveDashboard() {
  localStorage.setItem(DASHBOARD_KEY, JSON.stringify({ widgets: state.widgets }));
}

function newWidget(type, overrides = {}) {
  const preset = WIDGET_TYPES.find((item) => item.type === type) || WIDGET_TYPES[0];
  return {
    id: window.crypto?.randomUUID ? window.crypto.randomUUID() : `widget-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    type,
    title: overrides.title || preset.label,
    query: overrides.query || preset.query,
    w: Number(overrides.w || preset.w || 3),
    h: Number(overrides.h || preset.h || 2),
  };
}

function saveSettings() {
  state.settings = {
    apiUrl: normalizeBaseURL(els.apiUrl.value),
    apiKey: els.apiKey.value.trim(),
    rangeMode: els.rangeMode.value,
    rangeFrom: els.rangeFrom.value,
    rangeTo: els.rangeTo.value,
    refreshInterval: Number(els.refreshInterval.value),
    showCustomMetrics: Boolean(els.showCustomMetrics.checked),
    theme: state.settings.theme || "dark",
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state.settings));
  scheduleRefresh();
  syncCustomMetricsVisibility();
}

function applyTheme(theme) {
  const normalized = theme === "light" ? "light" : "dark";
  state.settings.theme = normalized;
  document.body.classList.toggle("theme-light", normalized === "light");
  if (els.themeToggleIcon) {
    els.themeToggleIcon.textContent = normalized === "dark" ? "☀" : "☾";
  }
  if (els.themeToggle) {
    const label = normalized === "dark" ? "Ativar tema claro" : "Ativar tema escuro";
    els.themeToggle.setAttribute("title", label);
    els.themeToggle.setAttribute("aria-label", label);
  }
  requestAnimationFrame(() => {
    renderHourlyChart();
    renderWidgets();
  });
}

function normalizeBaseURL(value) {
  return String(value || "http://localhost:8080").trim().replace(/\/+$/, "");
}

function requestHeaders() {
  const headers = { "Content-Type": "application/json" };
  if (state.settings.apiKey) {
    headers.Authorization = `Bearer ${state.settings.apiKey}`;
    headers["X-API-Key"] = state.settings.apiKey;
  }
  return headers;
}

function endpoint(path, params = {}) {
  const url = new URL(path, state.settings.apiUrl);
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== "") url.searchParams.set(key, value);
  });
  return url;
}

async function getJSON(path, params = {}) {
  const response = await fetch(endpoint(path, params), { headers: requestHeaders() });
  if (!response.ok) throw new Error(`${response.status} ${response.statusText}`);
  return response.json();
}

function timeWindow() {
  if (state.settings.rangeMode === "custom") {
    const from = state.settings.rangeFrom ? new Date(state.settings.rangeFrom) : new Date(Date.now() - 15 * 60 * 1000);
    const to = state.settings.rangeTo ? new Date(state.settings.rangeTo) : new Date();
    return { from: from.toISOString(), to: to.toISOString() };
  }
  const amount = Number(state.settings.rangeMode.match(/\d+/)?.[0] || 15);
  const unit = state.settings.rangeMode.replace(String(amount), "");
  const multiplier = unit === "h" ? 60 * 60 * 1000 : 60 * 1000;
  const to = new Date();
  const from = new Date(to.getTime() - amount * multiplier);
  return { from: from.toISOString(), to: to.toISOString() };
}

function previousTimeWindow(window) {
  const to = new Date(window.from);
  const from = new Date(to.getTime() - Math.max(60_000, new Date(window.to) - new Date(window.from)));
  return { from: from.toISOString(), to: to.toISOString() };
}

async function refreshData() {
  saveSettings();
  try {
    const window = timeWindow();
    const previousWindow = previousTimeWindow(window);
    const chartWindow = hourlyChartWindow();
    const [records, previousRecords, chartRecords] = await Promise.all([
      getJSON("/v1/metrics/events", {
        ...window,
        limit: 1000,
      }),
      getJSON("/v1/metrics/events", {
        ...previousWindow,
        limit: 1000,
      }),
      getJSON("/v1/metrics/events", {
        ...chartWindow,
        limit: 5000,
      }),
    ]);
    state.events = records.map((record) => record.event || record);
    state.processes = groupProcessEvents(records);
    state.previousProcesses = groupProcessEvents(previousRecords);
    state.chartProcesses = groupProcessEvents(chartRecords);
    setStatus(true);
    renderAll();
  } catch (error) {
    setStatus(false, error.message);
  }
}

function groupProcessEvents(records) {
  const groups = new Map();
  records
    .map((record) => ({ record, event: record.event || record }))
    .sort((a, b) => new Date(a.event.timestamp) - new Date(b.event.timestamp))
    .forEach(({ record, event }) => {
    if (!String(event.name || "").startsWith("routing_slip.")) return;
    const tags = event.tags || {};
    const key = processExecutionKey(record, event, tags);
    if (!groups.has(key)) {
      groups.set(key, {
        id: key,
        executionKey: key,
        workflow: event.workflow || "-",
        messageId: tags.message_id || "-",
        correlationId: tags.correlation_id || "-",
        traceId: event.trace_id || tags.trace_id || "-",
        startedAt: event.timestamp,
        updatedAt: event.timestamp,
        tags: {},
        events: [],
        completed: 0,
        failed: 0,
        stopped: 0,
        totalSteps: Number(tags.total_steps || 0),
      });
    }
    const group = groups.get(key);
    group.events.push(event);
    group.workflow = event.workflow || group.workflow;
    group.messageId = tags.message_id || group.messageId;
    group.correlationId = tags.correlation_id || group.correlationId;
    group.traceId = event.trace_id || tags.trace_id || group.traceId;
    group.totalSteps = Math.max(group.totalSteps, Number(tags.total_steps || 0));
    group.startedAt = earlier(group.startedAt, event.timestamp);
    group.updatedAt = later(group.updatedAt, event.timestamp);
    Object.assign(group.tags, tags);
  });

  return [...groups.values()]
    .map((group) => {
      group.events.sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));
      summarizeProcessGroup(group);
      return group;
    })
    .sort((a, b) => new Date(b.updatedAt) - new Date(a.updatedAt));
}

function processExecutionKey(record, event, tags) {
  const base = tags.correlation_id || tags.message_id || event.workflow || "process";
  const trace = event.trace_id || tags.trace_id;
  if (trace) return `${base}::trace:${trace}`;
  const run = tags.run_id;
  const attempt = tags.attempt;
  if (run && attempt) return `${base}::run:${run}::attempt:${attempt}`;
  if (base !== "process") return `${base}::legacy`;
  return `process::record:${record.id || event.timestamp}`;
}

function summarizeProcessGroup(group) {
  const terminalByStep = new Map();
  group.events.forEach((event) => {
    if (!["routing_slip.step.completed", "routing_slip.step.failed", "routing_slip.step.stopped"].includes(event.name)) {
      return;
    }
    const tags = event.tags || {};
    const key = `${tags.step_index || "0"}:${event.step || tags.handler || event.name}`;
    const current = terminalByStep.get(key);
    if (!current || new Date(event.timestamp) >= new Date(current.timestamp)) {
      terminalByStep.set(key, event);
    }
  });

  group.completed = 0;
  group.failed = 0;
  group.stopped = 0;
  terminalByStep.forEach((event) => {
    if (event.name === "routing_slip.step.completed" || event.status === "success") group.completed += 1;
    if (event.name === "routing_slip.step.failed" || event.status === "failed") group.failed += 1;
    if (event.name === "routing_slip.step.stopped" || event.status === "stopped") group.stopped += 1;
  });

  const expected = group.totalSteps || inferExpectedSteps(group);
  group.status = group.failed > 0 ? "failed" : group.stopped > 0 ? "stopped" : group.completed >= expected ? "completed" : "running";
  group.expectedSteps = expected;
  group.remaining = Math.max(expected - group.completed - group.failed - group.stopped, 0);
  group.durationMs = Math.max(0, new Date(group.updatedAt) - new Date(group.startedAt));
}

function inferExpectedSteps(group) {
  const indexes = group.events.map((event) => Number((event.tags || {}).step_index || 0));
  return Math.max(...indexes, group.completed + group.failed + group.stopped, 1);
}

function renderAll() {
  const window = timeWindow();
  els.lastUpdate.textContent = `Atualizado ${formatDateTime(new Date().toISOString())}`;
  const chartWindow = hourlyChartWindow();
  els.chartSubtitle.textContent = `Ultimas 24 horas`;
  renderKpis();
  renderHourlyChart();
  renderTopWorkflows();
  renderWidgets();
  renderProcesses();
}

function renderKpis() {
  const current = kpiSnapshot(state.processes);
  const previous = kpiSnapshot(state.previousProcesses);
  setKpi(els.kpiTotal, els.kpiTotalSub, current.total, previous.total);
  setKpi(els.kpiSuccess, els.kpiSuccessSub, current.success, previous.success);
  setKpi(els.kpiErrors, els.kpiErrorsSub, current.failed, previous.failed, { lowerIsBetter: true });
  setKpi(els.kpiReprocess, els.kpiReprocessSub, current.reprocesses, previous.reprocesses, { lowerIsBetter: true });
}

function kpiSnapshot(processes) {
  return {
    total: processes.length,
    success: processes.filter((item) => item.status === "completed").length,
    failed: processes.filter((item) => item.status === "failed" || item.status === "stopped").length,
    reprocesses: processes.filter(isReprocess).length,
  };
}

function setKpi(valueElement, trendElement, current, previous, options = {}) {
  valueElement.textContent = formatMetricNumber(current);
  const trend = kpiTrend(current, previous, options);
  trendElement.className = `kpi-trend ${trend.direction} ${trend.tone}`;
  trendElement.innerHTML = `<span>${escapeHTML(trend.icon)} ${escapeHTML(trend.value)}</span> <small>vs periodo anterior</small>`;
}

function kpiTrend(current, previous, options = {}) {
  if (!previous && !current) return { direction: "neutral", tone: "neutral", icon: "→", value: "0%" };
  if (!previous) return { direction: "up", tone: options.lowerIsBetter ? "bad" : "good", icon: "↗", value: "+100%" };
  const delta = current - previous;
  const percent = (delta / previous) * 100;
  const sign = percent > 0 ? "+" : "";
  const direction = delta > 0 ? "up" : delta < 0 ? "down" : "neutral";
  const improved = options.lowerIsBetter ? delta < 0 : delta > 0;
  return {
    direction,
    tone: delta === 0 ? "neutral" : improved ? "good" : "bad",
    icon: delta > 0 ? "↗" : delta < 0 ? "↘" : "→",
    value: `${sign}${new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 }).format(percent)}%`,
  };
}

function renderWidgetPalette() {
  els.widgetPaletteList.innerHTML = WIDGET_TYPES.map((item) => `
    <div class="widget-palette-item" draggable="true" data-widget-type="${escapeHTML(item.type)}">
      <span class="widget-palette-icon">${escapeHTML(item.icon)}</span>
      <div>
        <span>${escapeHTML(item.label)}</span>
        <em>${escapeHTML(item.query)}</em>
      </div>
    </div>
  `).join("");
  els.widgetPaletteList.querySelectorAll("[data-widget-type]").forEach((item) => {
    item.addEventListener("dragstart", (event) => {
      event.dataTransfer.setData("text/widget-type", item.dataset.widgetType);
      event.dataTransfer.effectAllowed = "copy";
    });
  });
}

function renderWidgets() {
  if (!state.widgets.length) {
    els.metricsGrid.innerHTML = `<div class="metrics-empty">Ative o modo de edicao e arraste widgets para este grid.</div>`;
    return;
  }
  els.metricsGrid.innerHTML = state.widgets.map((widget) => widgetMarkup(widget)).join("");
  els.metricsGrid.querySelectorAll(".metric-widget").forEach((element) => bindWidgetElement(element));
  state.widgets.forEach((widget) => renderWidgetBody(widget));
}

function widgetMarkup(widget) {
  return `
    <article class="metric-widget" draggable="${state.editMode ? "true" : "false"}" data-widget-id="${escapeHTML(widget.id)}" style="--widget-w:${widget.w};--widget-h:${widget.h}">
      <header class="metric-widget-head">
        <div class="metric-widget-title">
          <strong>${escapeHTML(widget.title)}</strong>
          <code>${escapeHTML(widget.query)}</code>
        </div>
        <div class="metric-widget-actions">
          <button class="widget-action" type="button" data-widget-edit="${escapeHTML(widget.id)}" title="Editar" aria-label="Editar">✎</button>
          <button class="widget-action" type="button" data-widget-delete="${escapeHTML(widget.id)}" title="Excluir" aria-label="Excluir">🗑</button>
        </div>
      </header>
      <div class="metric-widget-body" id="widget-body-${escapeHTML(widget.id)}"></div>
      <span class="widget-resize" data-widget-resize="${escapeHTML(widget.id)}" aria-hidden="true"></span>
    </article>
  `;
}

function bindWidgetElement(element) {
  const widgetId = element.dataset.widgetId;
  element.addEventListener("dragstart", (event) => {
    if (!state.editMode) return event.preventDefault();
    state.draggedWidgetId = widgetId;
    event.dataTransfer.setData("text/widget-id", widgetId);
    event.dataTransfer.effectAllowed = "move";
  });
  element.addEventListener("dragover", (event) => {
    if (!state.editMode) return;
    event.preventDefault();
    element.classList.add("drag-over");
  });
  element.addEventListener("dragleave", () => element.classList.remove("drag-over"));
  element.addEventListener("drop", (event) => {
    if (!state.editMode) return;
    event.preventDefault();
    element.classList.remove("drag-over");
    const sourceId = event.dataTransfer.getData("text/widget-id");
    if (!sourceId || sourceId === widgetId) return;
    moveWidgetBefore(sourceId, widgetId);
  });
  element.querySelector("[data-widget-edit]")?.addEventListener("click", () => openWidgetEditor(widgetId));
  element.querySelector("[data-widget-delete]")?.addEventListener("click", () => deleteWidget(widgetId));
  element.querySelector("[data-widget-resize]")?.addEventListener("mousedown", (event) => startWidgetResize(event, widgetId));
}

function renderWidgetBody(widget, target = document.getElementById(`widget-body-${widget.id}`)) {
  if (!target) return;
  const result = evaluateWidgetQuery(widget.query, widget.type);
  target.innerHTML = "";
  if (result.error) {
    target.innerHTML = `<p class="metric-note">${escapeHTML(result.error)}</p>`;
    return;
  }
  if (widget.type === "query_value") {
    target.innerHTML = `<div class="metric-value"><strong>${escapeHTML(formatMetricNumber(result.value))}</strong><span>${escapeHTML(result.label)}</span></div>`;
    return;
  }
  if (widget.type === "table") {
    target.innerHTML = widgetTable(result.rows || []);
    return;
  }
  if (widget.type === "top_list") {
    target.innerHTML = widgetTopList(result.rows || []);
    return;
  }
  if (!result.series && Number.isFinite(result.value)) {
    result.series = [{ label: result.label || widget.title, value: result.value }];
  }
  if (!result.points) result.points = processPoints(filterQueryProcesses(state.processes, parseMetricQuery(widget.query).filters));
  if (!result.series) result.series = [];
  const canvas = document.createElement("canvas");
  canvas.className = "widget-canvas";
  target.appendChild(canvas);
  drawWidgetChart(canvas, widget.type, result);
}

function widgetTable(rows) {
  const visible = rows.slice(0, 8);
  if (!visible.length) return `<p class="metric-note">Sem dados para a query.</p>`;
  return `
    <table class="widget-table">
      <thead><tr><th>Data</th><th>Workflow</th><th>Status</th><th>Duração</th></tr></thead>
      <tbody>
        ${visible.map((item) => `<tr><td>${escapeHTML(formatDateTime(item.updatedAt))}</td><td>${escapeHTML(item.workflow)}</td><td>${escapeHTML(statusLabel(item.status))}</td><td>${escapeHTML(formatDuration(item.durationMs))}</td></tr>`).join("")}
      </tbody>
    </table>`;
}

function widgetTopList(rows) {
  if (!rows.length) return `<p class="metric-note">Sem dados para a query.</p>`;
  return `
    <table class="widget-top-list">
      <tbody>${rows.slice(0, 8).map((row, index) => `<tr><td>${index + 1}. ${escapeHTML(row.label)}</td><td><strong>${escapeHTML(formatMetricNumber(row.value))}</strong></td></tr>`).join("")}</tbody>
    </table>`;
}

function drawWidgetChart(canvas, type, result) {
  const context = prepareCanvas(canvas);
  const rect = canvas.getBoundingClientRect();
  context.clearRect(0, 0, rect.width, rect.height);
  if (type === "pie") return drawPie(context, rect, result.series);
  if (type === "point") return drawPoints(context, rect, result.points);
  if (type === "timeseries") return drawLine(context, rect, result.series);
  return drawBars(context, rect, result.series);
}

function renderHourlyChart() {
  const buckets = hourlyBuckets(state.chartProcesses);
  const canvas = els.hourlyChart;
  const context = prepareCanvas(canvas);
  const rect = canvas.getBoundingClientRect();
  const pad = { top: 18, right: 18, bottom: 34, left: 48 };
  const width = rect.width - pad.left - pad.right;
  const height = rect.height - pad.top - pad.bottom;
  context.clearRect(0, 0, rect.width, rect.height);
  drawChartArea(context, pad, width, height);
  drawDashedGrid(context, pad, width, height);

  if (buckets.length === 0) {
    context.fillStyle = cssVar("--muted");
    context.fillText("Sem processamentos nas ultimas 24h", pad.left + 8, pad.top + 24);
    els.chartPeak.textContent = "Sem pico";
    els.chartSubtitle.textContent = "Ultimas 24 horas - 0 total";
    return;
  }

  const peakValue = Math.max(...buckets.map((bucket) => bucket.value), 1);
  const max = Math.max(1, Math.ceil(peakValue * 1.18));
  const peak = buckets.find((bucket) => bucket.value === peakValue);
  const total = buckets.reduce((sum, bucket) => sum + bucket.value, 0);
  els.chartSubtitle.textContent = `Ultimas 24 horas - ${formatMetricNumber(total)} total`;
  els.chartPeak.textContent = `Peak: ${peakValue} @ ${peak?.label || "-"}`;

  const points = buckets.map((bucket, index) => ({
    x: pad.left + (width / Math.max(buckets.length - 1, 1)) * index,
    y: pad.top + height - (bucket.value / max) * height,
    label: bucket.label,
    value: bucket.value,
  }));

  const areaGradient = context.createLinearGradient(0, pad.top, 0, pad.top + height);
  areaGradient.addColorStop(0, "rgba(59,154,240,.18)");
  areaGradient.addColorStop(1, "rgba(59,154,240,0)");
  context.beginPath();
  context.moveTo(points[0].x, pad.top + height);
  drawSmoothPath(context, points);
  context.lineTo(points.at(-1).x, pad.top + height);
  context.closePath();
  context.fillStyle = areaGradient;
  context.fill();

  context.beginPath();
  drawSmoothPath(context, points);
  context.strokeStyle = cssVar("--blue");
  context.lineWidth = 1.35;
  context.stroke();

  context.fillStyle = cssVar("--muted");
  [0, Math.round(max * 0.25), Math.round(max * 0.5), Math.round(max * 0.75), max].forEach((value, index) => {
    const y = pad.top + height - (height / 4) * index;
    context.fillText(String(value), 12, y + 4);
  });
  points.forEach((point, index) => {
    if (index % 3 === 0 || index === points.length - 1) {
      context.fillText(point.label.replace(":00", "h"), point.x - 8, pad.top + height + 20);
    }
  });
}

function renderTopWorkflows() {
  const rows = topRows(state.processes, "workflow").slice(0, 6);
  const paddedRows = [...rows];
  while (paddedRows.length < 6) paddedRows.push({ label: "", value: 0, empty: true });
  const max = Math.max(...rows.map((row) => row.value), 1);
  els.workflowTopList.innerHTML = paddedRows.map((row) => `
    <div class="workflow-top-row ${row.empty ? "empty-workflow-row" : ""}">
      <strong title="${escapeHTML(row.label)}">${escapeHTML(row.label)}</strong>
      <div class="workflow-bar"><i style="width:${row.empty ? 0 : Math.max(9, (row.value / max) * 100)}%"></i></div>
      <span>${row.empty ? "" : escapeHTML(formatMetricNumber(row.value))}</span>
    </div>
  `).join("") + `
    <div class="workflow-scale" aria-hidden="true">
      <span>0</span>
      <span>${escapeHTML(formatMetricNumber(Math.round(max / 3)))}</span>
      <span>${escapeHTML(formatMetricNumber(Math.round((max / 3) * 2)))}</span>
      <span>${escapeHTML(formatMetricNumber(max))}</span>
    </div>
  `;
}

function hourlyBuckets(processes) {
  const window = hourlyChartWindow();
  const from = floorHour(new Date(window.from));
  const to = floorHour(new Date(window.to));
  const buckets = [];
  for (let cursor = new Date(from); cursor <= to; cursor = new Date(cursor.getTime() + 60 * 60 * 1000)) {
    buckets.push({ key: cursor.toISOString(), label: formatHour(cursor), value: 0, failed: 0 });
  }
  const index = new Map(buckets.map((bucket) => [bucket.key, bucket]));
  processes.forEach((process) => {
    const key = floorHour(new Date(process.startedAt)).toISOString();
    const bucket = index.get(key);
    if (!bucket) return;
    bucket.value += 1;
    if (process.status === "failed") bucket.failed += 1;
  });
  return buckets;
}

function hourlyChartWindow() {
  const to = new Date();
  const from = new Date(to.getTime() - 24 * 60 * 60 * 1000);
  return { from: from.toISOString(), to: to.toISOString() };
}

function renderProcesses() {
  const filtered = filterProcesses(state.processes, els.processFilter.value.trim());
  const completed = filtered.filter((item) => item.status === "completed").length;
  const failed = filtered.filter((item) => item.status === "failed").length;
  const running = filtered.filter((item) => item.status === "running").length;
  els.processSummary.textContent = `${filtered.length} processos, ${completed} concluidos, ${failed} falhas, ${running} em execucao`;

  if (filtered.length === 0) {
    els.processList.innerHTML = `<div class="empty">Nenhum processamento encontrado no periodo.</div>`;
    return;
  }

  els.processList.innerHTML = filtered.map(processRow).join("");
  els.processList.querySelectorAll("[data-process-id]").forEach((row) => {
    row.addEventListener("click", () => openProcess(row.dataset.processId));
  });
}

function processRow(group) {
  const traceOrCorrelation = group.traceId && group.traceId !== "-" ? group.traceId : group.correlationId;
  const change = processChangeLabel(group);
  return `
    <button class="process-row" type="button" data-process-id="${escapeHTML(group.id)}">
      <code class="process-id-cell" title="${escapeHTML(traceOrCorrelation)}">${escapeHTML(shortIdentifier(traceOrCorrelation))}</code>
      <span class="process-workflow-cell" title="${escapeHTML(group.workflow)}">${escapeHTML(group.workflow)}</span>
      <span class="duration-cell">${escapeHTML(formatDuration(group.durationMs))}</span>
      <span class="process-change ${change.tone}">${escapeHTML(change.icon)} ${escapeHTML(change.label)}</span>
      <span class="status ${group.status}">${escapeHTML(statusLabel(group.status))}</span>
      <time>${escapeHTML(formatDateOnly(group.updatedAt))}</time>
    </button>
  `;
}

function processChangeLabel(group) {
  if (group.status === "failed") return { tone: "bad", icon: "↘", label: `${group.failed || 1} falha(s)` };
  if (group.status === "stopped") return { tone: "warn", icon: "→", label: `${group.remaining} faltante(s)` };
  if (group.status === "running") return { tone: "warn", icon: "→", label: `${group.completed}/${group.expectedSteps}` };
  return { tone: "good", icon: "↗", label: `${group.completed}/${group.expectedSteps}` };
}

function filterProcesses(processes, rawFilter) {
  if (!rawFilter) return processes;
  const filters = rawFilter.split(/[,\s]+/).map(parseAttributeFilter).filter(Boolean);
  if (filters.length === 0) return processes;
  return processes.filter((process) =>
    filters.every(({ key, value }) => String(processSearchFields(process)[key] || "").toLowerCase().includes(value.toLowerCase())),
  );
}

function parseAttributeFilter(text) {
  const [key, ...rest] = String(text).split(":");
  const value = rest.join(":");
  if (!key || !value) return null;
  return { key: key.trim(), value: value.trim() };
}

function processSearchFields(process) {
  return {
    workflow: process.workflow,
    message_id: process.messageId,
    correlation_id: process.correlationId,
    trace_id: process.traceId,
    status: process.status,
    ...process.tags,
  };
}

function openProcess(id) {
  const process = state.processes.find((item) => item.id === id);
  if (!process) return;
  els.modalTitle.textContent = process.workflow;
  els.modalCopy.textContent = `${process.correlationId} - ${statusLabel(process.status)} - ${formatDuration(process.durationMs)}`;
  els.modalStatusPill.className = `status ${process.status}`;
  els.modalStatusPill.textContent = statusLabel(process.status);
  const metadata = [
    ["Workflow", process.workflow],
    ["Status", statusLabel(process.status)],
    ["Correlation ID", process.correlationId],
    ["Trace ID", process.traceId],
    ["Message ID", process.messageId],
    ["Inicio", formatDateTime(process.startedAt)],
    ["Fim", formatDateTime(process.updatedAt)],
    ["Tempo total", formatDuration(process.durationMs)],
  ];
  const tagEntries = Object.entries(process.tags || {}).sort(([a], [b]) => a.localeCompare(b));
  const steps = processSteps(process);
  els.modalBody.innerHTML = `
    <section class="process-attribute-grid">
      ${metadata.map(([label, value]) => metaItem(label, value)).join("")}
      <div class="process-attribute-steps">
        <span>Etapas</span>
        <strong>${process.completed}/${process.expectedSteps}</strong>
      </div>
    </section>
    <section class="process-step-strip">
      <p>Etapas de processamento</p>
      <span>${steps.length} etapa(s) - ${escapeHTML(formatDuration(process.durationMs))} total</span>
      <span>${process.failed} erro(s)</span>
    </section>
    <section class="step-table-head" aria-hidden="true">
      <span>#</span>
      <span>Data / hora</span>
      <span>Tipo de etapa</span>
      <span>Duração</span>
      <span>Status</span>
      <span></span>
    </section>
    <section class="step-list">
      ${steps.map((step, index) => stepItem(step, index)).join("")}
    </section>
    <footer class="process-modal-footer">
      <span>Trace: <strong>${escapeHTML(process.traceId)}</strong></span>
      <span>Correlation: <strong>${escapeHTML(process.correlationId)}</strong></span>
      ${tagEntries.length ? `<span>${tagEntries.length} tag(s) capturada(s)</span>` : ""}
    </footer>
  `;
  els.modalBody.querySelectorAll("[data-step-index]").forEach((button) => {
    button.addEventListener("click", () => button.closest(".step-item")?.classList.toggle("open"));
  });
  els.processModal.showModal();
}

function processSteps(process) {
  const groups = new Map();
  process.events.forEach((event) => {
    const tags = event.tags || {};
    const key = `${tags.step_index || "0"}:${event.step || tags.handler || event.name}`;
    if (!groups.has(key)) {
      groups.set(key, {
        index: Number(tags.step_index || 0),
        step: event.step || tags.handler || event.name,
        handler: tags.handler || event.step || "-",
        status: "running",
        startedAt: event.timestamp,
        updatedAt: event.timestamp,
        duration: "",
        input: tags.input_value || "",
        rule: tags.rule_applied || "",
        output: tags.output_value || "",
        failure: tags.failure_reason || "",
        events: [],
      });
    }
    const step = groups.get(key);
    step.events.push(event);
    step.startedAt = earlier(step.startedAt, event.timestamp);
    step.updatedAt = later(step.updatedAt, event.timestamp);
    if (tags.duration_ms) step.duration = `${tags.duration_ms} ms`;
    if (tags.input_value) step.input = tags.input_value;
    if (tags.rule_applied) step.rule = tags.rule_applied;
    if (tags.output_value) step.output = tags.output_value;
    if (tags.failure_reason) step.failure = tags.failure_reason;
    if (["success", "failed", "stopped"].includes(event.status)) {
      step.status = event.status;
      if (event.status === "success") step.failure = "";
    }
  });
  return [...groups.values()].sort((a, b) => a.index - b.index || new Date(a.startedAt) - new Date(b.startedAt));
}

function stepItem(step, index) {
  return `
    <article class="step-item ${step.status}">
      <button class="step-summary" type="button" data-step-index="${index}">
        <span class="step-index">${String(index + 1).padStart(2, "0")}</span>
        <time>${escapeHTML(formatDateTime(step.startedAt))}</time>
        <span class="step-name">
          <strong>${escapeHTML(step.step)}</strong>
          <span>${escapeHTML(step.handler)}</span>
        </span>
        <span class="step-duration">${escapeHTML(step.duration || "-")}</span>
        <span class="status ${step.status}">${escapeHTML(statusLabel(step.status))}</span>
        <span class="chevron">›</span>
      </button>
      <section class="step-details">
        <div class="step-detail-block"><h3>Input</h3>${formatDetail(step.input)}</div>
        <div class="step-detail-block"><h3>Regra aplicada</h3>${formatDetail(step.rule)}</div>
        <div class="step-detail-block"><h3>Output</h3>${formatDetail(step.output)}</div>
        <div class="step-detail-block"><h3>Erro</h3>${formatDetail(step.failure || "sem erros", step.failure ? "error-box" : "")}</div>
      </section>
    </article>
  `;
}

function syncRangeControls() {
  const visible = els.rangeMode.value === "custom";
  els.customRange.hidden = !visible;
  els.customRange.classList.toggle("visible", visible);
}

function syncCustomMetricsVisibility() {
  const visible = Boolean(state.settings.showCustomMetrics);
  document.querySelector("#metrics-builder")?.classList.toggle("hidden", !visible);
  els.editDashboardToggle?.classList.toggle("hidden", !visible);
  if (!visible && state.editMode) setDashboardEditMode(false);
}

function scheduleRefresh() {
  clearInterval(state.timer);
  if (state.settings.refreshInterval > 0) {
    state.timer = setInterval(refreshData, state.settings.refreshInterval);
  }
}

function setStatus(ok, detail = "") {
  els.dot.className = `dot ${ok ? "ok" : "fail"}`;
  els.statusText.textContent = ok ? "Online" : `Offline ${detail}`;
}

function statusLabel(status) {
  return { completed: "OK", success: "OK", failed: "Erro", stopped: "Parado", running: "Em andamento" }[status] || status || "-";
}

function formatDetail(value, className = "") {
  if (!value) return "-";
  return `<pre class="detail-code ${escapeHTML(className)}">${escapeHTML(prettyJSON(value))}</pre>`;
}

function prettyJSON(value) {
  if (value === null || value === undefined) return "-";
  if (typeof value === "object") {
    return JSON.stringify(value, null, 2);
  }
  const text = String(value);
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return normalizeMultilineText(text);
  }
}

function normalizeMultilineText(value) {
  const text = String(value);
  try {
    const reparsed = JSON.parse(`"${text.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`);
    return reparsed.replace(/\r\n/g, "\n");
  } catch {
    return text
      .replace(/\\r\\n/g, "\n")
      .replace(/\\n/g, "\n")
      .replace(/\\t/g, "  ");
  }
}

function metaItem(label, value) {
  return `
    <div>
      <span>${escapeHTML(label)}</span>
      <strong title="${escapeHTML(value ?? "-")}">${escapeHTML(value ?? "-")}</strong>
    </div>
  `;
}

function formatDateTime(value) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("pt-BR", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(value));
}

function formatDateOnly(value) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("pt-BR", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  }).format(new Date(value)).replace(".", "");
}

function shortIdentifier(value) {
  const text = String(value || "-");
  if (text.length <= 16) return text;
  if (/^[0-9a-f]{32}$/i.test(text)) return `${text.slice(0, 8)}...${text.slice(-6)}`;
  return `${text.slice(0, 10)}...${text.slice(-6)}`;
}

function formatHour(value) {
  return new Intl.DateTimeFormat("pt-BR", { hour: "2-digit", minute: "2-digit" }).format(value);
}

function formatDuration(ms) {
  if (!Number.isFinite(ms) || ms <= 0) return "0 ms";
  if (ms < 1000) return `${ms} ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)} s`;
  return `${Math.floor(ms / 60_000)}m ${Math.round((ms % 60_000) / 1000)}s`;
}

function earlier(a, b) {
  return new Date(a) <= new Date(b) ? a : b;
}

function later(a, b) {
  return new Date(a) >= new Date(b) ? a : b;
}

function floorHour(value) {
  const date = new Date(value);
  date.setMinutes(0, 0, 0);
  return date;
}

function prepareCanvas(canvas) {
  const ratio = window.devicePixelRatio || 1;
  const rect = canvas.getBoundingClientRect();
  canvas.width = Math.max(1, Math.floor(rect.width * ratio));
  canvas.height = Math.max(1, Math.floor(rect.height * ratio));
  const context = canvas.getContext("2d");
  context.scale(ratio, ratio);
  context.font = "12px Inter, system-ui, sans-serif";
  return context;
}

function cssVar(name) {
  const bodyValue = getComputedStyle(document.body).getPropertyValue(name).trim();
  return bodyValue || getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

function percentLabel(value, total) {
  if (!total) return "0%";
  return `${new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 }).format((value / total) * 100)}%`;
}

function roundRect(context, x, y, width, height, radius) {
  const safeRadius = Math.min(radius, Math.abs(width) / 2, Math.abs(height) / 2);
  context.beginPath();
  context.moveTo(x + safeRadius, y);
  context.arcTo(x + width, y, x + width, y + height, safeRadius);
  context.arcTo(x + width, y + height, x, y + height, safeRadius);
  context.arcTo(x, y + height, x, y, safeRadius);
  context.arcTo(x, y, x + width, y, safeRadius);
  context.closePath();
}

function drawGrid(context, pad, width, height) {
  context.strokeStyle = cssVar("--line");
  context.lineWidth = 1;
  context.beginPath();
  for (let i = 0; i <= 4; i += 1) {
    const y = pad.top + (height / 4) * i;
    context.moveTo(pad.left, y);
    context.lineTo(pad.left + width, y);
  }
  context.stroke();
}

function drawChartArea(context, pad, width, height) {
  const gradient = context.createLinearGradient(0, pad.top, 0, pad.top + height);
  gradient.addColorStop(0, "rgba(59,154,240,.11)");
  gradient.addColorStop(0.45, "rgba(59,154,240,.045)");
  gradient.addColorStop(1, "rgba(59,154,240,0)");
  context.fillStyle = gradient;
  context.fillRect(pad.left, pad.top, width, height);
}

function drawDashedGrid(context, pad, width, height) {
  context.save();
  context.strokeStyle = colorMix(cssVar("--line"), 0.28);
  context.lineWidth = 1;
  context.setLineDash([2, 6]);
  context.beginPath();
  for (let i = 0; i <= 4; i += 1) {
    const y = pad.top + (height / 4) * i;
    context.moveTo(pad.left, y);
    context.lineTo(pad.left + width, y);
  }
  for (let i = 0; i <= 6; i += 1) {
    const x = pad.left + (width / 6) * i;
    context.moveTo(x, pad.top);
    context.lineTo(x, pad.top + height);
  }
  context.stroke();
  context.restore();
}

function drawSmoothPath(context, points) {
  if (!points.length) return;
  context.moveTo(points[0].x, points[0].y);
  for (let index = 0; index < points.length - 1; index += 1) {
    const current = points[index];
    const next = points[index + 1];
    const midpointX = (current.x + next.x) / 2;
    context.bezierCurveTo(midpointX, current.y, midpointX, next.y, next.x, next.y);
  }
}

function colorMix(color, alpha) {
  if (color.startsWith("rgba")) return color.replace(/rgba\(([^)]+),\s*[^)]+\)/, `rgba($1, ${alpha})`);
  if (color.startsWith("rgb")) return color.replace("rgb(", "rgba(").replace(")", `, ${alpha})`);
  return color;
}

function evaluateWidgetQuery(rawQuery, widgetType) {
  const parsed = parseMetricQuery(rawQuery);
  const processes = filterQueryProcesses(state.processes, parsed.filters);
  if (parsed.metric === "reprocesses") {
    const reprocesses = processes.filter(isReprocess);
    return seriesResult(parsed, reprocesses, "Reprocessamentos");
  }
  if (parsed.action === "avg" && parsed.metric === "duration_ms") {
    return { value: average(processes.map((item) => item.durationMs)), label: "Duração média" };
  }
  if (parsed.action === "p95" && parsed.metric === "duration_ms") {
    return { value: percentile(processes.map((item) => item.durationMs), 95), label: "P95 duração" };
  }
  if (parsed.action === "top") {
    const key = parsed.metric || "workflow";
    return { rows: topRows(processes, key) };
  }
  if (parsed.action === "table" || widgetType === "table") {
    return { rows: processes };
  }
  if (parsed.groupBy) {
    return { series: topRows(processes, parsed.groupBy).map((row) => ({ label: row.label, value: row.value })) };
  }
  return seriesResult(parsed, processes, "Processamentos");
}

function parseMetricQuery(rawQuery) {
  const query = String(rawQuery || "count:processes{*}").trim();
  const match = query.match(/^([a-z_]+):([a-zA-Z0-9_.-]+)\{([^}]*)\}(?:\.rollup\(([^)]*)\))?/);
  if (!match) return { action: "count", metric: "processes", filters: {}, rollupSeconds: 3600 };
  const filters = {};
  let groupBy = "";
  String(match[3] || "*").split(",").map((item) => item.trim()).filter(Boolean).forEach((item) => {
    if (item === "*") return;
    const [key, ...rest] = item.split(":");
    const value = rest.join(":");
    if (key === "group_by") groupBy = value;
    else if (key && value) filters[key] = value;
  });
  const rollup = String(match[4] || "").split(",").map((part) => part.trim());
  return {
    action: match[1],
    metric: match[2],
    filters,
    groupBy,
    rollupSeconds: Number(rollup[1] || 3600),
  };
}

function filterQueryProcesses(processes, filters) {
  return processes.filter((process) =>
    Object.entries(filters || {}).every(([key, expected]) => String(processSearchFields(process)[key] ?? "").toLowerCase() === String(expected).toLowerCase()),
  );
}

function seriesResult(parsed, processes, label) {
  if (parsed.rollupSeconds && parsed.rollupSeconds < 3600) {
    return { series: timeBuckets(processes, parsed.rollupSeconds), value: processes.length, label };
  }
  return { series: statusSeries(processes), points: processPoints(processes), value: processes.length, label };
}

function statusSeries(processes) {
  const statuses = ["completed", "failed", "stopped", "running"];
  return statuses.map((status) => ({ label: statusLabel(status), value: processes.filter((item) => item.status === status).length }));
}

function timeBuckets(processes, seconds) {
  const window = timeWindow();
  const size = Math.max(60, Number(seconds || 3600)) * 1000;
  const from = new Date(window.from);
  const to = new Date(window.to);
  const buckets = [];
  for (let cursor = new Date(from); cursor <= to; cursor = new Date(cursor.getTime() + size)) {
    buckets.push({ start: cursor, label: seconds < 3600 ? formatHour(cursor) : formatDateTime(cursor.toISOString()), value: 0 });
  }
  processes.forEach((process) => {
    const index = Math.floor((new Date(process.startedAt) - from) / size);
    if (buckets[index]) buckets[index].value += 1;
  });
  return buckets;
}

function processPoints(processes) {
  return processes.map((item, index) => ({ x: index + 1, y: item.durationMs, status: item.status }));
}

function topRows(processes, key) {
  const counts = new Map();
  processes.forEach((process) => {
    const value = processSearchFields(process)[key] || "-";
    counts.set(value, (counts.get(value) || 0) + 1);
  });
  return [...counts.entries()].map(([label, value]) => ({ label, value })).sort((a, b) => b.value - a.value || a.label.localeCompare(b.label));
}

function isReprocess(process) {
  const tags = process.tags || {};
  return ["reprocess", "reprocessed", "reprocessar"].some((key) => String(tags[key] || "").toLowerCase() === "true")
    || process.events.some((event) => /reprocess/i.test(event.name || "") || /reprocess/i.test((event.tags || {}).event || ""));
}

function average(values) {
  const list = values.filter((value) => Number.isFinite(value));
  if (!list.length) return 0;
  return Math.round(list.reduce((sum, value) => sum + value, 0) / list.length);
}

function percentile(values, percent) {
  const list = values.filter((value) => Number.isFinite(value)).sort((a, b) => a - b);
  if (!list.length) return 0;
  return list[Math.min(list.length - 1, Math.ceil((percent / 100) * list.length) - 1)];
}

function drawBars(context, rect, series) {
  const pad = { top: 14, right: 10, bottom: 24, left: 28 };
  const width = rect.width - pad.left - pad.right;
  const height = rect.height - pad.top - pad.bottom;
  drawGrid(context, pad, width, height);
  const max = Math.max(...series.map((item) => item.value), 1);
  const gap = 8;
  const barWidth = Math.max(12, (width - gap * (series.length - 1)) / Math.max(series.length, 1));
  series.forEach((item, index) => {
    const x = pad.left + index * (barWidth + gap);
    const barHeight = (item.value / max) * height;
    context.fillStyle = [cssVar("--green"), cssVar("--red"), cssVar("--primary"), cssVar("--blue")][index % 4];
    roundRect(context, x, pad.top + height - barHeight, barWidth, barHeight, 4);
    context.fill();
    context.fillStyle = cssVar("--muted");
    context.fillText(String(item.label).slice(0, 8), x, pad.top + height + 17);
  });
}

function drawLine(context, rect, series) {
  const pad = { top: 14, right: 12, bottom: 24, left: 28 };
  const width = rect.width - pad.left - pad.right;
  const height = rect.height - pad.top - pad.bottom;
  drawGrid(context, pad, width, height);
  const max = Math.max(...series.map((item) => item.value), 1);
  context.beginPath();
  series.forEach((item, index) => {
    const x = pad.left + (width / Math.max(series.length - 1, 1)) * index;
    const y = pad.top + height - (item.value / max) * height;
    if (index === 0) context.moveTo(x, y);
    else context.lineTo(x, y);
  });
  context.strokeStyle = cssVar("--blue");
  context.lineWidth = 2;
  context.stroke();
}

function drawPoints(context, rect, points) {
  const pad = { top: 14, right: 12, bottom: 24, left: 34 };
  const width = rect.width - pad.left - pad.right;
  const height = rect.height - pad.top - pad.bottom;
  drawGrid(context, pad, width, height);
  const max = Math.max(...points.map((item) => item.y), 1);
  points.slice(-80).forEach((item, index, list) => {
    const x = pad.left + (width / Math.max(list.length - 1, 1)) * index;
    const y = pad.top + height - (item.y / max) * height;
    context.fillStyle = item.status === "failed" ? cssVar("--red") : cssVar("--blue");
    context.beginPath();
    context.arc(x, y, 3, 0, Math.PI * 2);
    context.fill();
  });
}

function drawPie(context, rect, series) {
  const total = Math.max(series.reduce((sum, item) => sum + item.value, 0), 1);
  const radius = Math.max(20, Math.min(rect.width, rect.height) / 2 - 18);
  const cx = rect.width / 2;
  const cy = rect.height / 2;
  let start = -Math.PI / 2;
  series.forEach((item, index) => {
    const slice = (item.value / total) * Math.PI * 2;
    context.fillStyle = [cssVar("--green"), cssVar("--red"), cssVar("--primary"), cssVar("--blue"), cssVar("--purple")][index % 5];
    context.beginPath();
    context.moveTo(cx, cy);
    context.arc(cx, cy, radius, start, start + slice);
    context.closePath();
    context.fill();
    start += slice;
  });
}

function formatMetricNumber(value) {
  if (value >= 1000 && value < 1_000_000) return new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 }).format(value);
  return new Intl.NumberFormat("pt-BR").format(value || 0);
}

function setDashboardEditMode(enabled) {
  state.editMode = Boolean(enabled);
  document.body.classList.toggle("dashboard-editing", state.editMode);
  els.editDashboardToggle.classList.toggle("editing", state.editMode);
  els.editDashboardToggle.setAttribute("aria-pressed", String(state.editMode));
  els.dashboardEditHint.textContent = state.editMode ? "Modo edicao: arraste, redimensione e configure widgets" : "Modo visualizacao";
  renderWidgets();
}

function moveWidgetBefore(sourceId, targetId) {
  const sourceIndex = state.widgets.findIndex((item) => item.id === sourceId);
  const targetIndex = state.widgets.findIndex((item) => item.id === targetId);
  if (sourceIndex < 0 || targetIndex < 0) return;
  const [widget] = state.widgets.splice(sourceIndex, 1);
  const nextTargetIndex = state.widgets.findIndex((item) => item.id === targetId);
  state.widgets.splice(nextTargetIndex, 0, widget);
  saveDashboard();
  renderWidgets();
}

function addWidget(type) {
  state.widgets.push(newWidget(type));
  saveDashboard();
  renderWidgets();
}

function deleteWidget(widgetId) {
  state.widgets = state.widgets.filter((item) => item.id !== widgetId);
  saveDashboard();
  renderWidgets();
}

function openWidgetEditor(widgetId) {
  const widget = state.widgets.find((item) => item.id === widgetId);
  if (!widget) return;
  state.editingWidgetId = widgetId;
  els.widgetEditorTitle.textContent = `Editar ${widget.title}`;
  els.widgetTitleInput.value = widget.title;
  els.widgetTypeInput.value = widget.type;
  els.widgetQueryInput.value = widget.query;
  renderWidgetPreview();
  els.widgetEditorModal.showModal();
}

function renderWidgetPreview() {
  const widget = {
    id: "preview",
    title: els.widgetTitleInput.value || "Preview",
    type: els.widgetTypeInput.value,
    query: els.widgetQueryInput.value,
    w: 4,
    h: 2,
  };
  els.widgetPreviewBody.innerHTML = `<div class="metric-widget" style="height:220px"><div class="metric-widget-body" id="widget-preview-target"></div></div>`;
  renderWidgetBody(widget, document.querySelector("#widget-preview-target"));
}

function saveWidgetEditor() {
  const widget = state.widgets.find((item) => item.id === state.editingWidgetId);
  if (!widget) return;
  widget.title = els.widgetTitleInput.value.trim() || widget.title;
  widget.type = els.widgetTypeInput.value;
  widget.query = els.widgetQueryInput.value.trim() || widget.query;
  saveDashboard();
  renderWidgets();
}

function startWidgetResize(event, widgetId) {
  event.preventDefault();
  event.stopPropagation();
  const widget = state.widgets.find((item) => item.id === widgetId);
  if (!widget) return;
  const element = document.querySelector(`[data-widget-id="${CSS.escape(widgetId)}"]`);
  const gridStyle = getComputedStyle(els.metricsGrid);
  const columnCount = getComputedStyle(els.metricsGrid).gridTemplateColumns.split(" ").filter(Boolean).length || 12;
  const gap = Number.parseFloat(gridStyle.gap) || 10;
  const rowHeight = Number.parseFloat(gridStyle.getPropertyValue("--cell")) || 96;
  const columnWidth = (els.metricsGrid.clientWidth - gap * (columnCount - 1)) / columnCount;
  state.resizing = {
    widget,
    element,
    x: event.clientX,
    y: event.clientY,
    w: widget.w,
    h: widget.h,
    columnUnit: Math.max(24, columnWidth + gap),
    rowUnit: Math.max(24, rowHeight + gap),
  };
  document.body.classList.add("widget-resizing");
  document.addEventListener("mousemove", resizeWidget);
  document.addEventListener("mouseup", stopWidgetResize, { once: true });
}

function resizeWidget(event) {
  if (!state.resizing) return;
  event.preventDefault();
  const dx = Math.round((event.clientX - state.resizing.x) / state.resizing.columnUnit);
  const dy = Math.round((event.clientY - state.resizing.y) / state.resizing.rowUnit);
  state.resizing.widget.w = Math.max(2, Math.min(12, state.resizing.w + dx));
  state.resizing.widget.h = Math.max(1, Math.min(8, state.resizing.h + dy));
  if (state.resizing.element) {
    state.resizing.element.style.setProperty("--widget-w", state.resizing.widget.w);
    state.resizing.element.style.setProperty("--widget-h", state.resizing.widget.h);
  }
}

function stopWidgetResize() {
  document.removeEventListener("mousemove", resizeWidget);
  document.body.classList.remove("widget-resizing");
  state.resizing = null;
  saveDashboard();
  renderWidgets();
}

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;" })[char]);
}

function openSettingsPanel() {
  els.configModal.classList.add("open");
  els.configModal.setAttribute("aria-hidden", "false");
}

function closeSettingsPanel() {
  els.configModal.classList.remove("open");
  els.configModal.setAttribute("aria-hidden", "true");
}

els.openConfig.addEventListener("click", openSettingsPanel);
document.querySelector(".settings-close")?.addEventListener("click", closeSettingsPanel);
els.saveConfig.addEventListener("click", () => {
  saveSettings();
  closeSettingsPanel();
  refreshData();
});
els.refreshNow.addEventListener("click", refreshData);
els.themeToggle?.addEventListener("click", () => {
  applyTheme(state.settings.theme === "dark" ? "light" : "dark");
  saveSettings();
});
els.rangeMode.addEventListener("change", () => {
  syncRangeControls();
  refreshData();
});
els.rangeFrom.addEventListener("change", refreshData);
els.rangeTo.addEventListener("change", refreshData);
els.processSearch.addEventListener("click", renderProcesses);
els.processFilter.addEventListener("input", renderProcesses);
els.processFilter.addEventListener("keydown", (event) => {
  if (event.key === "Enter") renderProcesses();
});
els.modalClose.addEventListener("click", () => els.processModal.close());
els.editDashboardToggle.addEventListener("click", () => setDashboardEditMode(!state.editMode));
els.metricsGrid.addEventListener("dragover", (event) => {
  if (!state.editMode) return;
  if (event.dataTransfer.types.includes("text/widget-type")) {
    event.preventDefault();
    event.dataTransfer.dropEffect = "copy";
  }
});
els.metricsGrid.addEventListener("drop", (event) => {
  if (!state.editMode) return;
  const type = event.dataTransfer.getData("text/widget-type");
  if (!type) return;
  event.preventDefault();
  addWidget(type);
});
els.widgetPreviewRefresh.addEventListener("click", renderWidgetPreview);
els.widgetSave.addEventListener("click", saveWidgetEditor);
[els.widgetTitleInput, els.widgetTypeInput, els.widgetQueryInput].forEach((input) => {
  input.addEventListener("input", renderWidgetPreview);
});
[els.processModal, els.widgetEditorModal].forEach((modal) => {
  modal.addEventListener("click", (event) => {
    if (event.target === modal) modal.close();
  });
});
window.addEventListener("keydown", (event) => {
  if (event.key === "Escape") closeSettingsPanel();
});
window.addEventListener("resize", () => {
  renderHourlyChart();
  renderWidgets();
});

loadSettings();
loadDashboard();
renderWidgetPalette();
scheduleRefresh();
refreshData();
