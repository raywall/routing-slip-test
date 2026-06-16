const state = {
  mocks: [],
  selectedId: "",
  generatorCatalog: [],
};

const fields = [
  "name",
  "description",
  "method",
  "endpoint_pattern",
  "status_code",
  "response_mode",
  "latency_min_ms",
  "latency_max_ms",
  "expected_headers",
  "expected_query",
  "expected_body",
  "response_headers",
  "response_payload",
  "additional_variables",
  "generator_kind",
  "generator_config",
  "tags",
  "enabled",
];

window.addEventListener("DOMContentLoaded", async () => {
  bindActions();
  await Promise.all([loadGeneratorCatalog(), loadMocks()]);
  resetForm();
});

function bindActions() {
  document.getElementById("new-mock").addEventListener("click", resetForm);
  document.getElementById("save-mock").addEventListener("click", saveMock);
  document.getElementById("delete-mock").addEventListener("click", deleteMock);
  document.getElementById("generator_kind").addEventListener("change", renderGeneratorHint);
  fields.forEach((field) => {
    const element = document.getElementById(field);
    if (!element) return;
    element.addEventListener("input", updatePreview);
    element.addEventListener("change", updatePreview);
  });
}

async function loadGeneratorCatalog() {
  const response = await fetch("/api/catalog/generators");
  state.generatorCatalog = await response.json();
  renderGeneratorHint();
}

async function loadMocks() {
  const response = await fetch("/api/mocks");
  state.mocks = await response.json();
  renderMockList();
}

function renderMockList() {
  const container = document.getElementById("mock-list");
  container.innerHTML = "";
  if (!state.mocks.length) {
    container.innerHTML = `<p class="muted">Nenhum mock cadastrado ainda.</p>`;
    return;
  }
  state.mocks.forEach((mock) => {
    const card = document.createElement("button");
    card.type = "button";
    card.className = `mock-card ${mock.id === state.selectedId ? "active" : ""}`;
    card.innerHTML = `
      <h4>${escapeHTML(mock.name)}</h4>
      <div class="card-line">
        <span>${escapeHTML(mock.method)} ${escapeHTML(mock.endpoint_pattern)}</span>
        <span class="badge ${mock.enabled ? "enabled" : "disabled"}">${mock.enabled ? "ativo" : "desligado"}</span>
      </div>
      <div class="card-line">
        <span class="muted">${escapeHTML(mock.response_mode)}</span>
        <span class="muted">${mock.status_code}</span>
      </div>
    `;
    card.addEventListener("click", () => fillForm(mock));
    container.appendChild(card);
  });
}

function fillForm(mock) {
  state.selectedId = mock.id;
  document.getElementById("form-title").textContent = `Editar mock: ${mock.name}`;
  document.getElementById("delete-mock").disabled = false;
  document.getElementById("name").value = mock.name || "";
  document.getElementById("description").value = mock.description || "";
  document.getElementById("method").value = mock.method || "GET";
  document.getElementById("endpoint_pattern").value = mock.endpoint_pattern || "/";
  document.getElementById("status_code").value = mock.status_code || 200;
  document.getElementById("response_mode").value = mock.response_mode || "static";
  document.getElementById("latency_min_ms").value = mock.latency_min_ms || 0;
  document.getElementById("latency_max_ms").value = mock.latency_max_ms || 0;
  document.getElementById("expected_headers").value = pretty(mock.expected_headers || {});
  document.getElementById("expected_query").value = pretty(mock.expected_query || {});
  document.getElementById("expected_body").value = mock.expected_body == null ? "null" : pretty(mock.expected_body);
  document.getElementById("response_headers").value = pretty(mock.response_headers || {"Content-Type":"application/json"});
  document.getElementById("response_payload").value = pretty(
    mock.response_mode === "template" ? (mock.response_template || {}) : (mock.response_body || {})
  );
  document.getElementById("additional_variables").value = pretty(mock.additional_variables || {});
  document.getElementById("generator_kind").value = mock.generator?.kind || "";
  document.getElementById("generator_config").value = pretty(resolveGeneratorConfig(mock.generator) || {});
  document.getElementById("tags").value = (mock.tags || []).join(", ");
  document.getElementById("enabled").checked = !!mock.enabled;
  renderGeneratorHint();
  renderMockList();
  updatePreview();
}

function resolveGeneratorConfig(generator) {
  if (!generator) return {};
  if (generator.kind === "consignado_operacao_v1") return generator.consignado_operacao || {};
  if (generator.kind === "consignado_saldos_v1") return generator.consignado_saldos || {};
  return {};
}

function resetForm() {
  state.selectedId = "";
  document.getElementById("form-title").textContent = "Novo mock";
  document.getElementById("delete-mock").disabled = true;
  document.getElementById("name").value = "";
  document.getElementById("description").value = "";
  document.getElementById("method").value = "GET";
  document.getElementById("endpoint_pattern").value = "/";
  document.getElementById("status_code").value = 200;
  document.getElementById("response_mode").value = "static";
  document.getElementById("latency_min_ms").value = 0;
  document.getElementById("latency_max_ms").value = 0;
  document.getElementById("expected_headers").value = "{}";
  document.getElementById("expected_query").value = "{}";
  document.getElementById("expected_body").value = "null";
  document.getElementById("response_headers").value = "{\"Content-Type\":\"application/json\"}";
  document.getElementById("response_payload").value = "{}";
  document.getElementById("additional_variables").value = "{}";
  document.getElementById("generator_kind").value = "";
  document.getElementById("generator_config").value = "{\n  \"customer_path\": \"codigoCliente\",\n  \"contract_path\": \"identificadorOperacaoCredito\"\n}";
  document.getElementById("tags").value = "";
  document.getElementById("enabled").checked = true;
  renderGeneratorHint();
  renderMockList();
  updatePreview();
}

async function saveMock() {
  try {
    const payload = collectPayload();
    const url = state.selectedId ? `/api/mocks/${state.selectedId}` : "/api/mocks";
    const method = state.selectedId ? "PUT" : "POST";
    const response = await fetch(url, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    const body = await response.json();
    if (!response.ok) {
      throw new Error(body.error || "Falha ao salvar mock");
    }
    await loadMocks();
    fillForm(body);
  } catch (error) {
    alert(error.message);
  }
}

async function deleteMock() {
  if (!state.selectedId || !confirm("Deseja remover este mock?")) {
    return;
  }
  const response = await fetch(`/api/mocks/${state.selectedId}`, { method: "DELETE" });
  if (!response.ok) {
    const body = await response.json();
    alert(body.error || "Falha ao excluir");
    return;
  }
  await loadMocks();
  resetForm();
}

function collectPayload() {
  const mode = document.getElementById("response_mode").value;
  const generatorKind = document.getElementById("generator_kind").value;
  const generatorConfig = parseJSONField("generator_config");
  const generator = generatorKind ? buildGenerator(generatorKind, generatorConfig) : undefined;
  const payload = {
    name: document.getElementById("name").value.trim(),
    description: document.getElementById("description").value.trim(),
    method: document.getElementById("method").value,
    endpoint_pattern: document.getElementById("endpoint_pattern").value.trim(),
    status_code: Number(document.getElementById("status_code").value),
    response_mode: mode,
    latency_min_ms: Number(document.getElementById("latency_min_ms").value || 0),
    latency_max_ms: Number(document.getElementById("latency_max_ms").value || 0),
    expected_headers: parseJSONField("expected_headers"),
    expected_query: parseJSONField("expected_query"),
    expected_body: parseJSONField("expected_body"),
    response_headers: parseJSONField("response_headers"),
    additional_variables: parseJSONField("additional_variables"),
    enabled: document.getElementById("enabled").checked,
    tags: document.getElementById("tags").value
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean),
  };
  const responsePayload = parseJSONField("response_payload");
  if (mode === "template") {
    payload.response_template = responsePayload;
  } else if (mode === "static") {
    payload.response_body = responsePayload;
  } else {
    payload.generator = generator;
  }
  return payload;
}

function buildGenerator(kind, config) {
  if (kind === "consignado_operacao_v1") {
    return { kind, consignado_operacao: config };
  }
  if (kind === "consignado_saldos_v1") {
    return { kind, consignado_saldos: config };
  }
  return { kind };
}

function renderGeneratorHint() {
  const kind = document.getElementById("generator_kind").value;
  const hint = document.getElementById("generator-description");
  const item = state.generatorCatalog.find((entry) => entry.kind === kind);
  hint.textContent = item ? item.description : "Use templates para respostas determinísticas ou um gerador para montar payloads randômicos coerentes.";
}

function updatePreview() {
  try {
    const payload = collectPayload();
    document.getElementById("preview").textContent = [
      `${payload.method} ${payload.endpoint_pattern}`,
      "",
      JSON.stringify(payload, null, 2),
      "",
      "Exemplo de placeholders:",
      '{ "contrato": "{{path.identificadorOperacaoCredito}}", "cliente": "{{query.codigoCliente}}", "matricula": "{{body.data.codigo_matricula}}" }',
    ].join("\n");
  } catch (error) {
    document.getElementById("preview").textContent = `JSON inválido: ${error.message}`;
  }
}

function parseJSONField(id) {
  const raw = document.getElementById(id).value.trim();
  if (!raw) {
    return null;
  }
  return JSON.parse(raw);
}

function pretty(value) {
  return JSON.stringify(value, null, 2);
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
