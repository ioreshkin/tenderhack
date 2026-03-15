const state = {
  selected: null,
  batchItems: [],
  lots: [],
  activeLotID: "",
  regions: [],
  recentDocuments: [],
  lastRequest: null,
  lastResponse: null,
  lastBatchRequest: null,
  lastBatchResponse: null,
  batchPanelVisible: false,
  replacingSelection: false,
  importedMeta: "",
  suggestController: null,
  searchController: null,
  suggestToken: 0,
  searchToken: 0,
  suggestCache: new Map(),
  searchCache: new Map(),
  suggestionItems: [],
  searchResultItems: [],
  searchSelectionIDs: new Set(),
  activeSuggestionIndex: -1,
  suggestionSignature: "",
  resultsSignature: "",
  isComposing: false,
  hintAlternatives: [],
  manualEntrySeq: 0,
};

const LOTS_STORAGE_KEY = "tenderhack-lots-v1";

const els = {
  contentGrid: document.querySelector("#content-grid"),
  queryInput: document.querySelector("#query"),
  suggestions: document.querySelector("#suggestions"),
  searchAddManualBtn: document.querySelector("#search-add-manual-btn"),
  searchMeta: document.querySelector("#search-meta"),
  queryHints: document.querySelector("#query-hints"),
  searchBtn: document.querySelector("#search-btn"),
  searchResultsPanel: document.querySelector("#search-results-panel"),
  searchResultsMeta: document.querySelector("#search-results-meta"),
  searchResults: document.querySelector("#search-results"),
  addSearchSelectionBtn: document.querySelector("#add-search-selection-btn"),
  searchSession: document.querySelector("#search-session"),
  searchSessionText: document.querySelector("#search-session-text"),
  selectedPanel: document.querySelector("#selected-panel"),
  selectedName: document.querySelector("#selected-name"),
  selectedMeta: document.querySelector("#selected-meta"),
  selectedIDTag: document.querySelector("#selected-id-tag"),
  selectedCategoryTag: document.querySelector("#selected-category-tag"),
  selectedManufacturerTag: document.querySelector("#selected-manufacturer-tag"),
  selectedQuantity: document.querySelector("#selected-quantity"),
  changeItemBtn: document.querySelector("#change-item-btn"),
  restoreSelectionBtn: document.querySelector("#restore-selection-btn"),
  addToBatchBtn: document.querySelector("#add-to-batch-btn"),
  newLotBtn: document.querySelector("#new-lot-btn"),
  lotDrafts: document.querySelector("#lot-drafts"),
  batchPanel: document.querySelector("#batch-panel"),
  batchName: document.querySelector("#batch-name"),
  batchAddManualBtn: document.querySelector("#batch-add-manual-btn"),
  batchRegionSelect: document.querySelector("#batch-region"),
  batchMonthsBack: document.querySelector("#batch-months-back"),
  batchSettingsSimpleBtn: document.querySelector("#batch-settings-simple-btn"),
  batchSettingsAdvancedBtn: document.querySelector("#batch-settings-advanced-btn"),
  batchSettingsSimplePanel: document.querySelector("#batch-settings-simple-panel"),
  batchSettingsAdvancedPanel: document.querySelector("#batch-settings-advanced-panel"),
  batchTimeImportanceLabel: document.querySelector("#batch-time-importance-label"),
  batchSameRegionImportanceLabel: document.querySelector("#batch-same-region-importance-label"),
  batchOtherRegionImportanceLabel: document.querySelector("#batch-other-region-importance-label"),
  batchTimeDecay: document.querySelector("#batch-time-decay"),
  batchSameRegionWeight: document.querySelector("#batch-same-region-weight"),
  batchOtherRegionWeight: document.querySelector("#batch-other-region-weight"),
  batchCalcBtn: document.querySelector("#batch-calc-btn"),
  batchDocBtn: document.querySelector("#batch-doc-btn"),
  openBatchDocLink: document.querySelector("#open-batch-doc-link"),
  batchItems: document.querySelector("#batch-items"),
  batchSummary: document.querySelector("#batch-summary"),
  batchDisclosure: document.querySelector("#batch-disclosure"),
  batchResults: document.querySelector("#batch-results"),
  manualPanel: document.querySelector("#manual-panel"),
  manualEntries: document.querySelector("#manual-entries"),
  addManualEntryBtn: document.querySelector("#add-manual-entry-btn"),
  summaryPanel: document.querySelector("#summary-panel"),
  topPanel: document.querySelector("#top-panel"),
  stepsPanel: document.querySelector("#steps-panel"),
  resultsPanel: document.querySelector("#results-panel"),
  resultsBody: document.querySelector("#results-body"),
  summaryCards: document.querySelector("#summary-cards"),
  topRecommendations: document.querySelector("#top-recommendations"),
  steps: document.querySelector("#steps"),
  regionSelect: document.querySelector("#region"),
  calcStatus: document.querySelector("#calc-status"),
  calcBtn: document.querySelector("#calc-btn"),
  recalcBtn: document.querySelector("#recalc-btn"),
  docBtn: document.querySelector("#doc-btn"),
  openDocLink: document.querySelector("#open-doc-link"),
  recentDocs: document.querySelector("#recent-docs"),
  newCalcBtn: document.querySelector("#new-calc-btn"),
  monthsBack: document.querySelector("#months-back"),
  settingsSimpleBtn: document.querySelector("#settings-simple-btn"),
  settingsAdvancedBtn: document.querySelector("#settings-advanced-btn"),
  settingsSimplePanel: document.querySelector("#settings-simple-panel"),
  settingsAdvancedPanel: document.querySelector("#settings-advanced-panel"),
  timeImportanceLabel: document.querySelector("#time-importance-label"),
  sameRegionImportanceLabel: document.querySelector("#same-region-importance-label"),
  otherRegionImportanceLabel: document.querySelector("#other-region-importance-label"),
  timeDecay: document.querySelector("#time-decay"),
  sameRegionWeight: document.querySelector("#same-region-weight"),
  otherRegionWeight: document.querySelector("#other-region-weight"),
  windowHintPanel: document.querySelector("#window-hint-panel"),
};

let suggestTimer = null;

bindEvents();
bootstrap();

function bindEvents() {
  els.searchAddManualBtn.addEventListener("click", () => {
    clearSearchWorkspace();
    addManualBatchItem();
  });
  els.calcBtn.addEventListener("click", () => calculate());
  els.recalcBtn.addEventListener("click", () => calculate(getExcludedIDs()));
  els.docBtn.addEventListener("click", createDocument);
  els.newCalcBtn.addEventListener("click", resetWorkspace);
  els.newLotBtn.addEventListener("click", createNewLot);
  els.addManualEntryBtn.addEventListener("click", addManualEntryRow);
  els.changeItemBtn.addEventListener("click", () => beginReplaceSelection(true));
  els.restoreSelectionBtn.addEventListener("click", restoreCurrentSelection);
  els.addToBatchBtn.addEventListener("click", upsertCurrentSelectionInBatch);
  els.batchCalcBtn.addEventListener("click", calculateBatch);
  els.batchDocBtn.addEventListener("click", createBatchDocument);
  els.batchAddManualBtn.addEventListener("click", addManualBatchItem);
  els.batchName.addEventListener("input", () => {
    if (state.batchItems.length) {
      renderBatchSummary();
    }
    syncActiveLot();
  });
  els.settingsSimpleBtn.addEventListener("click", () => {
    setSettingsMode("simple", "main");
  });
  els.settingsAdvancedBtn.addEventListener("click", () => {
    setSettingsMode("advanced", "main");
  });
  els.batchSettingsSimpleBtn.addEventListener("click", () => {
    setSettingsMode("simple", "batch");
    syncMainSettingsFromBatch();
  });
  els.batchSettingsAdvancedBtn.addEventListener("click", () => {
    setSettingsMode("advanced", "batch");
    syncMainSettingsFromBatch();
  });

  els.queryInput.addEventListener("compositionstart", () => {
    state.isComposing = true;
  });
  els.queryInput.addEventListener("compositionend", () => {
    state.isComposing = false;
    scheduleSuggest();
  });
  els.queryInput.addEventListener("input", () => {
    if (state.isComposing) {
      return;
    }
    syncReplaceSelectionFromQuery();
    scheduleSuggest();
  });
  els.queryInput.addEventListener("focus", () => {
    if (state.suggestionItems.length) {
      showSuggestions();
    }
  });
  els.queryInput.addEventListener("blur", () => {
    window.setTimeout(hideSuggestions, 140);
  });
  els.queryInput.addEventListener("keydown", handleQueryKeydown);
  els.selectedQuantity.addEventListener("change", () => {
    els.selectedQuantity.value = formatIntegerInput(els.selectedQuantity.value || 1);
  });

  [
    els.batchRegionSelect,
    els.batchMonthsBack,
    els.batchTimeImportanceLabel,
    els.batchSameRegionImportanceLabel,
    els.batchOtherRegionImportanceLabel,
    els.batchTimeDecay,
    els.batchSameRegionWeight,
    els.batchOtherRegionWeight,
  ].forEach((node) => {
    node.addEventListener("change", syncMainSettingsFromBatch);
  });
}

function cloneValue(value) {
  return value ? JSON.parse(JSON.stringify(value)) : value;
}

function defaultLotSettings() {
  return {
    region: "",
    monthsBack: 1,
    mode: "simple",
    timeImportanceLabel: "важно",
    sameRegionImportanceLabel: "очень важно",
    otherRegionImportanceLabel: "средне",
    timeDecay: 0.65,
    sameRegionWeight: 1,
    otherRegionWeight: 0.4,
  };
}

function createLotDraft(name = "") {
  const nextNumber = (state.lots?.length || 0) + 1;
  return {
    id: `lot-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    name: String(name || `Лот ${nextNumber}`).trim(),
    batchItems: [],
    batchSettings: defaultLotSettings(),
    lastBatchRequest: null,
    lastBatchResponse: null,
    batchDocURL: "",
  };
}

function isManualBatchItem(item) {
  return Number(item?.cte_id || 0) <= 0;
}

function sanitizeBatchItem(item, lotID, index) {
  const manualOnly = isManualBatchItem(item);
  const baseID = String(item?.item_id || "").trim();
  const nextID = baseID || (manualOnly ? `manual-${lotID}-${index + 1}` : `cte-${Number(item?.cte_id || 0)}`);
  return {
    item_id: nextID,
    cte_id: manualOnly ? 0 : Number(item?.cte_id || 0),
    name: String(item?.name || (manualOnly ? `Позиция пользователя ${index + 1}` : "")).trim(),
    category: String(item?.category || "").trim(),
    manufacturer: String(item?.manufacturer || "").trim(),
    quantity: parsePositiveInteger(item?.quantity, 1),
    region: String(item?.region || "").trim(),
    months_back: 1,
    settings_mode: String(item?.settings_mode || "").trim(),
    time_importance_label: String(item?.time_importance_label || "").trim(),
    same_region_importance_label: String(item?.same_region_importance_label || "").trim(),
    other_region_importance_label: String(item?.other_region_importance_label || "").trim(),
    time_weight_mode: "linear_floor",
    time_decay: Number(item?.time_decay || 0.65),
    same_region_weight: Number(item?.same_region_weight || 1),
    other_region_weight: Number(item?.other_region_weight || 0.4),
    max_results: Number(item?.max_results || 150),
    excluded_ids: Array.isArray(item?.excluded_ids) ? item.excluded_ids : [],
    price_overrides: Array.isArray(item?.price_overrides) ? item.price_overrides : [],
    weight_overrides: Array.isArray(item?.weight_overrides) ? item.weight_overrides : [],
    manual_entries: Array.isArray(item?.manual_entries) ? item.manual_entries : [],
  };
}

function sanitizeLotDraft(rawLot, index) {
  const lot = rawLot || {};
  const next = createLotDraft(String(lot.name || `Лот ${index + 1}`));
  next.id = String(lot.id || next.id);
  next.batchSettings = {
    ...defaultLotSettings(),
    ...(lot.batchSettings || {}),
    monthsBack: 1,
  };
  const seenItemIDs = new Set();
  next.batchItems = (Array.isArray(lot.batchItems) ? lot.batchItems : [])
    .map((item, itemIndex) => sanitizeBatchItem(item, next.id, itemIndex))
    .filter((item) => {
      if (!item.item_id || seenItemIDs.has(item.item_id)) {
        return false;
      }
      seenItemIDs.add(item.item_id);
      if (isManualBatchItem(item)) {
        return item.manual_entries.length > 0 || item.name;
      }
      return item.cte_id > 0;
    });
  next.lastBatchRequest = lot.lastBatchRequest || null;
  next.lastBatchResponse = lot.lastBatchResponse || null;
  next.batchDocURL = String(lot.batchDocURL || "").trim();
  return next;
}

function persistLots() {
  window.localStorage.setItem(LOTS_STORAGE_KEY, JSON.stringify({
    activeLotID: state.activeLotID,
    lots: state.lots,
  }));
}

function getActiveLot() {
  return state.lots.find((lot) => lot.id === state.activeLotID) || null;
}

function clearSearchWorkspace() {
  state.selected = null;
  state.lastRequest = null;
  state.lastResponse = null;
  state.replacingSelection = false;
  state.suggestionItems = [];
  state.searchResultItems = [];
  state.searchSelectionIDs = new Set();
  state.activeSuggestionIndex = -1;
  state.suggestionSignature = "";
  state.resultsSignature = "";
  state.hintAlternatives = [];
  els.queryInput.value = "";
  els.selectedQuantity.value = 1;
  hideSuggestions();
  renderHintChips([]);
  hideSearchResults();
  updateDocLink("");
  renderManualEntries([]);
  resetComputedPanels();
  applyPanelVisibility();
}

function renderLotDrafts() {
  if (!els.lotDrafts) {
    return;
  }
  if (!state.lots.length) {
    els.lotDrafts.innerHTML = '<div class="empty-state">Пока нет лотов.</div>';
    return;
  }

  els.lotDrafts.innerHTML = state.lots.map((lot) => {
    const isActive = lot.id === state.activeLotID;
    const itemCount = Array.isArray(lot.batchItems) ? lot.batchItems.filter(batchItemHasData).length : 0;
    const total = Number(lot.lastBatchResponse?.summary?.grand_total || 0);
    return `
      <article class="lot-draft${isActive ? " lot-draft-active" : ""}">
        <button type="button" class="lot-draft-main" data-lot-switch="${escapeHtml(lot.id)}">
          <span class="recent-title">${escapeHtml(lot.name || "Лот НМЦК")}</span>
          <span class="recent-meta">${formatInt(itemCount)} поз. | ${lot.lastBatchResponse ? money(total) : "без расчета"}</span>
        </button>
        <button type="button" class="button button-ghost lot-draft-remove" data-lot-remove="${escapeHtml(lot.id)}">Удалить</button>
      </article>
    `;
  }).join("");

  els.lotDrafts.querySelectorAll("[data-lot-switch]").forEach((node) => {
    node.addEventListener("click", () => activateLot(node.getAttribute("data-lot-switch")));
  });
  els.lotDrafts.querySelectorAll("[data-lot-remove]").forEach((node) => {
    node.addEventListener("click", (event) => {
      event.stopPropagation();
      removeLot(node.getAttribute("data-lot-remove"));
    });
  });
}

function syncActiveLot() {
  const activeLot = getActiveLot();
  if (!activeLot) {
    return;
  }
  activeLot.name = String(els.batchName.value || "").trim() || activeLot.name || "Лот НМЦК";
  activeLot.batchItems = cloneValue(state.batchItems) || [];
  activeLot.batchSettings = collectBatchSettings();
  activeLot.lastBatchRequest = cloneValue(state.lastBatchRequest);
  activeLot.lastBatchResponse = cloneValue(state.lastBatchResponse);
  activeLot.batchDocURL = els.openBatchDocLink.classList.contains("hidden") ? "" : els.openBatchDocLink.href;
  persistLots();
  renderLotDrafts();
}

function loadLotIntoUI(lot, options = {}) {
  const { reveal = false } = options;
  state.activeLotID = lot.id;
  state.batchItems = cloneValue(lot.batchItems) || [];
  state.lastBatchRequest = cloneValue(lot.lastBatchRequest);
  state.lastBatchResponse = cloneValue(lot.lastBatchResponse);
  state.batchPanelVisible = Boolean(reveal && state.batchItems.length);
  els.batchName.value = lot.name || "Лот НМЦК";
  applySettingsToBatch(lot.batchSettings || defaultLotSettings());
  applySettingsToMain(lot.batchSettings || defaultLotSettings());
  renderBatchPanel();
  refreshVisibleSearchResults();
  updateBatchDocLink(lot.batchDocURL || "");
  renderLotDrafts();
  persistLots();
}

function activateLot(lotID) {
  syncActiveLot();
  const nextLot = state.lots.find((lot) => lot.id === lotID);
  if (!nextLot) {
    return;
  }
  if (nextLot.id === state.activeLotID) {
    return;
  }
  clearSearchWorkspace();
  loadLotIntoUI(nextLot, { reveal: nextLot.batchItems.length > 0 });
  if (nextLot.batchItems.length) {
    focusBatchPanel();
  }
  setStatus(`Активен лот: ${nextLot.name}`, "success");
}

function removeLot(lotID) {
  if (!lotID) {
    return;
  }
  const remaining = state.lots.filter((lot) => lot.id !== lotID);
  if (!remaining.length) {
    const emptyLot = createLotDraft("Лот 1");
    state.lots = [emptyLot];
    clearSearchWorkspace();
    loadLotIntoUI(emptyLot, { reveal: false });
    setStatus("Последний лот удален. Создан новый пустой лот.", "warning");
    return;
  }
  state.lots = remaining;
  if (state.activeLotID === lotID) {
    clearSearchWorkspace();
    loadLotIntoUI(remaining[0], { reveal: remaining[0].batchItems.length > 0 });
  } else {
    persistLots();
    renderLotDrafts();
  }
  setStatus("Лот удален", "success");
}

function createNewLot() {
  syncActiveLot();
  const lot = createLotDraft();
  state.lots.push(lot);
  clearSearchWorkspace();
  loadLotIntoUI(lot, { reveal: false });
  setStatus(`Создан новый лот: ${lot.name}`, "success");
}

function ensureLotsLoaded() {
  if (state.lots.length) {
    return;
  }
  try {
    const raw = window.localStorage.getItem(LOTS_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed?.lots) && parsed.lots.length) {
        state.lots = parsed.lots.map((lot, index) => sanitizeLotDraft(lot, index));
        state.activeLotID = parsed.activeLotID || state.lots[0].id;
      }
    }
  } catch (error) {
    state.lots = [];
    state.activeLotID = "";
  }
  if (!state.lots.length) {
    const firstLot = createLotDraft("Лот 1");
    state.lots = [firstLot];
    state.activeLotID = firstLot.id;
  } else {
    let emptyLot = state.lots.find((lot) => !Array.isArray(lot.batchItems) || !lot.batchItems.some(batchItemHasData));
    if (!emptyLot) {
      emptyLot = createLotDraft();
      state.lots.unshift(emptyLot);
    }
    state.activeLotID = emptyLot.id;
  }
  const activeLot = getActiveLot() || state.lots[0];
  loadLotIntoUI(activeLot, { reveal: false });
}

async function bootstrap() {
  try {
    const data = await fetchJSON("/api/bootstrap");
    state.regions = data.regions || [];
    state.recentDocuments = data.recent_documents || [];
    state.importedMeta = data.imported
      ? `Данные. Импортировано позиций СТЕ: ${formatInt(data.cte_count)}, закупок: ${formatInt(data.contract_count)}`
      : "База еще не импортирована. Сначала выполни import.";

    const regionOptions = '<option value="">Все регионы</option>' +
      state.regions.map((region) => `<option value="${escapeHtml(region)}">${escapeHtml(region)}</option>`).join("");
    els.regionSelect.innerHTML = regionOptions;
    els.batchRegionSelect.innerHTML = regionOptions;
    els.searchMeta.textContent = state.importedMeta;
    renderRecentDocuments();
    syncSettingsModePanels("both");
    ensureLotsLoaded();
  } catch (error) {
    els.searchMeta.textContent = "Не удалось загрузить стартовые данные.";
    setStatus("Ошибка загрузки bootstrap", "error");
  }
}

function getSettingsMode(scope = "main") {
  const advancedBtn = scope === "batch" ? els.batchSettingsAdvancedBtn : els.settingsAdvancedBtn;
  return advancedBtn.classList.contains("active") ? "advanced" : "simple";
}

function setSettingsMode(mode, scope = "both") {
  const normalized = mode === "advanced" ? "advanced" : "simple";
  if (scope === "main" || scope === "both") {
    els.settingsSimpleBtn.classList.toggle("active", normalized === "simple");
    els.settingsAdvancedBtn.classList.toggle("active", normalized === "advanced");
  }
  if (scope === "batch" || scope === "both") {
    els.batchSettingsSimpleBtn.classList.toggle("active", normalized === "simple");
    els.batchSettingsAdvancedBtn.classList.toggle("active", normalized === "advanced");
  }
  syncSettingsModePanels(scope);
}

function syncSettingsModePanels(scope = "both") {
  if (scope === "main" || scope === "both") {
    const mainMode = getSettingsMode("main");
    els.settingsSimplePanel.classList.toggle("hidden", mainMode !== "simple");
    els.settingsAdvancedPanel.classList.toggle("hidden", mainMode !== "advanced");
  }
  if (scope === "batch" || scope === "both") {
    const batchMode = getSettingsMode("batch");
    els.batchSettingsSimplePanel.classList.toggle("hidden", batchMode !== "simple");
    els.batchSettingsAdvancedPanel.classList.toggle("hidden", batchMode !== "advanced");
  }
}

function collectMainSettings() {
  return {
    region: els.regionSelect.value,
    monthsBack: 1,
    mode: getSettingsMode("main"),
    timeImportanceLabel: els.timeImportanceLabel.value,
    sameRegionImportanceLabel: els.sameRegionImportanceLabel.value,
    otherRegionImportanceLabel: els.otherRegionImportanceLabel.value,
    timeDecay: els.timeDecay.value,
    sameRegionWeight: els.sameRegionWeight.value,
    otherRegionWeight: els.otherRegionWeight.value,
  };
}

function collectBatchSettings() {
  return {
    region: els.batchRegionSelect.value,
    monthsBack: 1,
    mode: getSettingsMode("batch"),
    timeImportanceLabel: els.batchTimeImportanceLabel.value,
    sameRegionImportanceLabel: els.batchSameRegionImportanceLabel.value,
    otherRegionImportanceLabel: els.batchOtherRegionImportanceLabel.value,
    timeDecay: els.batchTimeDecay.value,
    sameRegionWeight: els.batchSameRegionWeight.value,
    otherRegionWeight: els.batchOtherRegionWeight.value,
  };
}

function pickValue(value, fallback) {
  return value ?? fallback;
}

function invalidateBatchResults() {
  state.lastBatchRequest = null;
  state.lastBatchResponse = null;
  updateBatchDocLink("");
}

function applySettingsToMain(settings) {
  els.regionSelect.value = settings.region || "";
  if (els.monthsBack) {
    els.monthsBack.value = 1;
  }
  els.timeImportanceLabel.value = settings.timeImportanceLabel || "важно";
  els.sameRegionImportanceLabel.value = settings.sameRegionImportanceLabel || "очень важно";
  els.otherRegionImportanceLabel.value = settings.otherRegionImportanceLabel || "средне";
  els.timeDecay.value = pickValue(settings.timeDecay, 0.65);
  els.sameRegionWeight.value = pickValue(settings.sameRegionWeight, 1);
  els.otherRegionWeight.value = pickValue(settings.otherRegionWeight, 0.4);
  setSettingsMode(settings.mode || "simple", "main");
}

function applySettingsToBatch(settings) {
  els.batchRegionSelect.value = settings.region || "";
  if (els.batchMonthsBack) {
    els.batchMonthsBack.value = 1;
  }
  els.batchTimeImportanceLabel.value = settings.timeImportanceLabel || "важно";
  els.batchSameRegionImportanceLabel.value = settings.sameRegionImportanceLabel || "очень важно";
  els.batchOtherRegionImportanceLabel.value = settings.otherRegionImportanceLabel || "средне";
  els.batchTimeDecay.value = pickValue(settings.timeDecay, 0.65);
  els.batchSameRegionWeight.value = pickValue(settings.sameRegionWeight, 1);
  els.batchOtherRegionWeight.value = pickValue(settings.otherRegionWeight, 0.4);
  setSettingsMode(settings.mode || "simple", "batch");
}

function applyGlobalSettingsToBatchItems() {
  if (!state.batchItems.length) {
    return;
  }
  const settings = collectBatchSettings();
  state.batchItems = state.batchItems.map((item) => ({
    ...item,
    region: settings.region,
    months_back: Number(pickValue(settings.monthsBack, 1)),
    settings_mode: settings.mode,
    time_importance_label: settings.timeImportanceLabel,
    same_region_importance_label: settings.sameRegionImportanceLabel,
    other_region_importance_label: settings.otherRegionImportanceLabel,
    time_decay: Number(pickValue(settings.timeDecay, 0.65)),
    same_region_weight: Number(pickValue(settings.sameRegionWeight, 1)),
    other_region_weight: Number(pickValue(settings.otherRegionWeight, 0.4)),
    manual_entries: Array.isArray(item.manual_entries)
      ? item.manual_entries.map((entry) => ({
          ...entry,
          region: settings.region || "",
        }))
      : [],
  }));
}

function syncMainSettingsFromBatch() {
  const hadCalculatedResult = Boolean(state.lastBatchResponse);
  applySettingsToMain(collectBatchSettings());
  applyGlobalSettingsToBatchItems();
  invalidateBatchResults();
  renderBatchPanel();
  syncActiveLot();
  if (hadCalculatedResult && state.batchItems.length) {
    setStatus("Параметры лота изменились. Пересчитай лот, чтобы обновить итог и PDF.", "warning");
  }
}

function scheduleSuggest() {
  window.clearTimeout(suggestTimer);
  suggestTimer = window.setTimeout(runSuggest, 140);
}

async function runSuggest() {
  const query = normalizeQuery(els.queryInput.value);
  if (!query) {
    state.suggestionItems = [];
    state.activeSuggestionIndex = -1;
    renderHintChips([]);
    hideSuggestions();
    return;
  }

  if (query.length < 1 && !/^\d+$/.test(query)) {
    hideSuggestions();
    return;
  }

  if (state.suggestCache.has(query)) {
    renderSuggestions(query, state.suggestCache.get(query));
    return;
  }

  if (state.suggestController) {
    state.suggestController.abort();
  }
  state.suggestController = new AbortController();
  const token = ++state.suggestToken;

  renderSuggestionLoading();

  try {
    const data = await fetchJSON(`/api/suggest?q=${encodeURIComponent(query)}`, state.suggestController.signal);
    if (token !== state.suggestToken) {
      return;
    }
    state.suggestCache.set(query, data);
    renderSuggestions(query, data);
  } catch (error) {
    if (error.name === "AbortError") {
      return;
    }
    hideSuggestions();
  }
}

function renderSuggestionLoading() {
  els.suggestions.innerHTML = '<div class="empty-state">Подбираю быстрые варианты...</div>';
  showSuggestions();
}

function renderSuggestions(query, data) {
  state.hintAlternatives = data.alternatives || [];
  renderHintChips(state.hintAlternatives);

  const items = data.items || [];
  const signature = JSON.stringify(items.map((item) => [item.id, item.score]).concat(state.hintAlternatives));
  if (signature === state.suggestionSignature && !els.suggestions.classList.contains("hidden")) {
    return;
  }
  state.suggestionSignature = signature;
  state.suggestionItems = items;
  state.activeSuggestionIndex = items.length ? 0 : -1;

  if (!items.length) {
    els.suggestions.innerHTML = '<div class="empty-state">По этому вводу быстрых подсказок нет. Нажми Enter для полного поиска.</div>';
    showSuggestions();
    return;
  }

  els.suggestions.innerHTML = items.map((item, index) => `
    <button type="button" class="suggestion-item${index === state.activeSuggestionIndex ? " active" : ""}" data-suggest-index="${index}">
      <span class="suggestion-title">${escapeHtml(item.name)}</span>
      <span class="suggestion-meta">${escapeHtml(item.category)} | ${escapeHtml(item.manufacturer || "производитель не указан")}</span>
      <span class="suggestion-score"><span class="confidence-chip" title="${escapeHtml(confidenceTooltip(item))}">Совпадение ${formatConfidence(item.confidence)}</span> | закупок ${formatInt(item.contract_count)} | ${escapeHtml(friendlySearchReason(item.match_reason))}</span>
    </button>
  `).join("");

  els.suggestions.querySelectorAll("[data-suggest-index]").forEach((node) => {
    node.addEventListener("mousedown", (event) => {
      event.preventDefault();
    });
    node.addEventListener("click", () => {
      const index = Number(node.getAttribute("data-suggest-index"));
      const item = state.suggestionItems[index];
      if (item) {
        els.queryInput.value = item.name;
        hideSuggestions();
        runFullSearch();
      }
    });
  });

  showSuggestions();
}

function renderHintChips(alternatives) {
  if (!alternatives || !alternatives.length) {
    els.queryHints.innerHTML = "";
    return;
  }

  els.queryHints.innerHTML = alternatives.map((item) => `
    <button type="button" class="hint-chip" data-alt="${escapeHtml(item)}">
      Возможно: ${escapeHtml(item)}
    </button>
  `).join("");

  els.queryHints.querySelectorAll("[data-alt]").forEach((node) => {
    node.addEventListener("click", () => {
      const value = node.getAttribute("data-alt") || "";
      els.queryInput.value = value;
      scheduleSuggest();
      runFullSearch();
    });
  });
}

function showSuggestions() {
  els.suggestions.classList.remove("hidden");
}

function hideSuggestions() {
  els.suggestions.classList.add("hidden");
}

function handleQueryKeydown(event) {
  if (event.key === "ArrowDown") {
    if (!state.suggestionItems.length) {
      return;
    }
    event.preventDefault();
    state.activeSuggestionIndex = (state.activeSuggestionIndex + 1) % state.suggestionItems.length;
    syncSuggestionActiveState();
    return;
  }

  if (event.key === "ArrowUp") {
    if (!state.suggestionItems.length) {
      return;
    }
    event.preventDefault();
    state.activeSuggestionIndex = state.activeSuggestionIndex <= 0
      ? state.suggestionItems.length - 1
      : state.activeSuggestionIndex - 1;
    syncSuggestionActiveState();
    return;
  }

  if (event.key === "Escape") {
    hideSuggestions();
    return;
  }

  if (event.key === "Enter") {
    event.preventDefault();
    hideSuggestions();
    runFullSearch();
  }
}

function syncSuggestionActiveState() {
  els.suggestions.querySelectorAll("[data-suggest-index]").forEach((node, index) => {
    node.classList.toggle("active", index === state.activeSuggestionIndex);
  });
}

async function runFullSearch() {
  const query = normalizeQuery(els.queryInput.value);
  hideSuggestions();
  if (!query) {
    setStatus("Введи название товара или код СТЕ", "warning");
    return;
  }
  if (query.length < 2 && !/^\d+$/.test(query)) {
    setStatus("Для полного поиска введи хотя бы 2 символа или код СТЕ", "warning");
    return;
  }

  state.selected = null;
  state.replacingSelection = false;
  renderSelected();
  resetComputedPanels();
  updateDocLink("");

  if (state.searchCache.has(query)) {
    renderSearchResults(query, state.searchCache.get(query));
    return;
  }

  if (state.searchController) {
    state.searchController.abort();
  }
  state.searchController = new AbortController();
  const token = ++state.searchToken;
  els.searchResultsPanel.classList.remove("hidden");
  els.searchResultsMeta.textContent = "Ищу полные результаты...";
  els.searchResults.innerHTML = '<div class="empty-state">Подбираю полный список совпадений...</div>';

  try {
    const data = await fetchJSON(`/api/search?q=${encodeURIComponent(query)}`, state.searchController.signal);
    if (token !== state.searchToken) {
      return;
    }
    state.searchCache.set(query, data);
    renderSearchResults(query, data);
  } catch (error) {
    if (error.name === "AbortError") {
      return;
    }
    els.searchResultsMeta.textContent = "";
    els.searchResults.innerHTML = '<div class="empty-state">Не удалось выполнить полный поиск.</div>';
    setStatus("Ошибка полного поиска", "error");
  }
}

function renderSearchResults(query, data) {
  const items = data.items || [];
  const alternatives = data.alternatives || [];
  state.searchResultItems = items;
  const availableIDs = new Set(items.map((item) => Number(item.id)));
  state.searchSelectionIDs = new Set(
    state.batchItems
      .map((item) => Number(item.cte_id))
      .filter((id) => availableIDs.has(id))
  );
  const signature = JSON.stringify({
    items: items.map((item) => [item.id, item.score, item.confidence]),
    alternatives,
    selected: Array.from(state.searchSelectionIDs).sort((a, b) => a - b),
  });
  if (signature === state.resultsSignature) {
    updateSearchSelectionAction();
    return;
  }
  state.resultsSignature = signature;

  els.searchResultsPanel.classList.remove("hidden");
  els.searchResultsMeta.textContent = alternatives.length
    ? `Найдено ${items.length}. Альтернативы: ${alternatives.join(", ")}`
    : `Найдено ${items.length} вариантов`;

  if (!items.length) {
    els.searchResults.innerHTML = '<div class="empty-state">Ничего не нашлось. Попробуй другое слово, бренд или числовой код СТЕ.</div>';
    updateSearchSelectionAction();
    return;
  }

  els.searchResults.innerHTML = items.map((item) => `
    <article class="search-result-item${state.searchSelectionIDs.has(Number(item.id)) ? " search-result-item-selected" : ""}" data-result-id="${item.id}">
      <label class="search-result-select" title="Галочка сразу добавляет товар в лот, снятие галочки удаляет его">
        <input type="checkbox" data-result-select="${item.id}" ${state.searchSelectionIDs.has(Number(item.id)) ? "checked" : ""}>
      </label>
      <div class="search-result-copy">
        <span class="result-title">${escapeHtml(item.name)}</span>
        <span class="result-meta">Код СТЕ ${item.id} | ${escapeHtml(item.category)} | ${escapeHtml(item.manufacturer || "производитель не указан")}</span>
        <span class="result-meta"><span class="confidence-chip" title="${escapeHtml(confidenceTooltip(item))}">Совпадение ${formatConfidence(item.confidence)}</span> | закупок ${formatInt(item.contract_count)} | ${escapeHtml(friendlySearchReason(item.match_reason))}</span>
        ${(Number(item.attribute_hits || 0) > 0 || Number(item.matched_name_tokens || 0) > 0 || state.batchItems.some((entry) => Number(entry.cte_id) === Number(item.id))) ? `
          <div class="row-badges">
            ${Number(item.matched_name_tokens || 0) > 0 ? `<span class="row-badge">в названии: ${formatInt(item.matched_name_tokens)}</span>` : ""}
            ${Number(item.attribute_hits || 0) > 0 ? `<span class="row-badge row-badge-success">в характеристиках: ${formatInt(item.attribute_hits)}</span>` : ""}
            ${state.batchItems.some((entry) => Number(entry.cte_id) === Number(item.id)) ? `<span class="row-badge row-badge-success">уже в лоте</span>` : ""}
          </div>
        ` : ""}
      </div>
    </article>
  `).join("");

  els.searchResults.querySelectorAll("[data-result-select]").forEach((node) => {
    node.addEventListener("change", () => {
      const resultID = Number(node.getAttribute("data-result-select"));
      const item = state.searchResultItems.find((entry) => Number(entry.id) === resultID);
      if (!item) {
        return;
      }
      if (node.checked) {
        upsertBatchItem(buildBatchItemFromSearchItem(item));
      } else {
        removeBatchItemByCTE(resultID);
      }
    });
    node.addEventListener("click", (event) => event.stopPropagation());
  });

  updateSearchSelectionAction();
}

function selectItem(item, source) {
  state.selected = item;
  state.replacingSelection = false;
  els.queryInput.value = item.name;
  els.selectedQuantity.value = 1;
  hideSuggestions();
  renderHintChips([]);
  state.suggestionItems = [];
  state.suggestionSignature = "";
  state.lastRequest = null;
  state.lastResponse = null;
  hideSearchResults();
  renderSelected();
  renderManualEntries([]);
  resetComputedPanels();
  updateDocLink("");
  applyPanelVisibility();
  setStatus(`Выбран товар с кодом СТЕ ${item.id}${source ? `, источник: ${source}` : ""}`, "success");
}

function renderSelected() {
  if (!state.selected) {
    els.selectedPanel.classList.add("hidden");
    els.manualPanel.classList.add("hidden");
    syncSearchSession();
    return;
  }

  els.selectedPanel.classList.remove("hidden");
  els.manualPanel.classList.remove("hidden");
  els.selectedName.textContent = state.selected.name;
  els.selectedMeta.textContent = `${state.selected.category} | ${state.selected.manufacturer || "производитель не указан"}`;
  els.selectedIDTag.textContent = `Код СТЕ ${state.selected.id}`;
  els.selectedCategoryTag.textContent = state.selected.category || "Категория не указана";
  els.selectedManufacturerTag.textContent = state.selected.manufacturer || "Производитель не указан";
  syncSearchSession();
}

function beginReplaceSelection(clearQuery = false) {
  if (!state.selected) {
    return;
  }
  state.replacingSelection = true;
  if (clearQuery) {
    els.queryInput.value = "";
  }
  hideSuggestions();
  hideSearchResults();
  applyPanelVisibility();
  els.queryInput.focus();
  if (!clearQuery) {
    els.queryInput.select();
  }
  setStatus(`Ищу другой товар вместо позиции СТЕ ${state.selected.id}. Текущий расчет временно скрыт.`, "warning");
}

function restoreCurrentSelection() {
  if (!state.selected) {
    return;
  }
  state.replacingSelection = false;
  els.queryInput.value = state.selected.name;
  hideSuggestions();
  hideSearchResults();
  applyPanelVisibility();
  setStatus(`Возвращена текущая позиция СТЕ ${state.selected.id}`, "success");
}

function syncReplaceSelectionFromQuery() {
  if (!state.selected) {
    return;
  }
  const query = normalizeQuery(els.queryInput.value);
  const selectedName = normalizeQuery(state.selected.name);
  const selectedID = String(state.selected.id || "");
  if (state.replacingSelection) {
    if (query === selectedName || query === selectedID) {
      state.replacingSelection = false;
      applyPanelVisibility();
    }
    return;
  }
  if (query && query !== selectedName && query !== selectedID) {
    state.replacingSelection = true;
    applyPanelVisibility();
  }
}

function applyPanelVisibility() {
  const hasSelected = Boolean(state.selected);
  const hasResults = Boolean(state.lastResponse);
  const showSelection = hasSelected && !state.replacingSelection;
  const showResults = hasResults && !state.replacingSelection;

  els.selectedPanel.classList.toggle("hidden", !showSelection);
  els.manualPanel.classList.toggle("hidden", !showSelection);
  els.summaryPanel.classList.toggle("hidden", !showResults);
  els.topPanel.classList.toggle("hidden", !showResults);
  els.stepsPanel.classList.toggle("hidden", !showResults);
  els.resultsPanel.classList.toggle("hidden", !showResults);
  syncSearchSession();
}

function syncSearchSession() {
  const active = Boolean(state.selected) && state.replacingSelection;
  els.searchSession.classList.toggle("hidden", !active);
  if (!active) {
    els.searchSessionText.textContent = "";
    return;
  }
  els.searchSessionText.textContent = `Сейчас идет новый поиск. Текущая позиция СТЕ ${state.selected.id} и нижние блоки скрыты, пока ты не выберешь другой товар или не вернешься к текущему.`;
}

function hideSearchResults() {
  els.searchResultsPanel.classList.add("hidden");
  els.searchResultsMeta.textContent = "";
  els.searchResults.innerHTML = "";
  state.searchResultItems = [];
  state.searchSelectionIDs = new Set();
  updateSearchSelectionAction();
}

function buildRequest(excluded = []) {
  const settingsMode = getSettingsMode();
  return {
    cte_id: Number(state.selected.id),
    region: els.regionSelect.value,
    months_back: Number(els.monthsBack.value || 1),
    settings_mode: settingsMode,
    time_importance_label: els.timeImportanceLabel.value,
    same_region_importance_label: els.sameRegionImportanceLabel.value,
    other_region_importance_label: els.otherRegionImportanceLabel.value,
    time_weight_mode: "linear_floor",
    time_decay: Number(els.timeDecay.value || 0.65),
    same_region_weight: Number(els.sameRegionWeight.value || 1),
    other_region_weight: Number(els.otherRegionWeight.value || 0.4),
    max_results: 150,
    excluded_ids: excluded,
    price_overrides: getPriceOverrides(),
    weight_overrides: getWeightOverrides(),
    manual_entries: getManualEntries(),
  };
}

function snapshotCurrentSelectionForBatch() {
  if (!state.selected) {
    return null;
  }
  const request = buildRequest(getExcludedIDs());
  const lotSettings = collectBatchSettings();
  return {
    item_id: `cte-${state.selected.id}`,
    cte_id: Number(state.selected.id),
    name: state.selected.name,
    category: state.selected.category || "",
    manufacturer: state.selected.manufacturer || "",
    quantity: parsePositiveInteger(els.selectedQuantity.value, 1),
    region: lotSettings.region,
    months_back: Number(pickValue(lotSettings.monthsBack, 1)),
    settings_mode: lotSettings.mode,
    time_importance_label: lotSettings.timeImportanceLabel,
    same_region_importance_label: lotSettings.sameRegionImportanceLabel,
    other_region_importance_label: lotSettings.otherRegionImportanceLabel,
    time_weight_mode: "linear_floor",
    time_decay: Number(pickValue(lotSettings.timeDecay, 0.65)),
    same_region_weight: Number(pickValue(lotSettings.sameRegionWeight, 1)),
    other_region_weight: Number(pickValue(lotSettings.otherRegionWeight, 0.4)),
    max_results: request.max_results,
    excluded_ids: request.excluded_ids || [],
    price_overrides: request.price_overrides || [],
    weight_overrides: request.weight_overrides || [],
    manual_entries: request.manual_entries || [],
  };
}

function buildBatchItemFromSearchItem(item) {
  const lotSettings = collectBatchSettings();
  const request = {
    region: lotSettings.region,
    months_back: Number(pickValue(lotSettings.monthsBack, 1)),
    settings_mode: lotSettings.mode,
    time_importance_label: lotSettings.timeImportanceLabel,
    same_region_importance_label: lotSettings.sameRegionImportanceLabel,
    other_region_importance_label: lotSettings.otherRegionImportanceLabel,
    time_weight_mode: "linear_floor",
    time_decay: Number(pickValue(lotSettings.timeDecay, 0.65)),
    same_region_weight: Number(pickValue(lotSettings.sameRegionWeight, 1)),
    other_region_weight: Number(pickValue(lotSettings.otherRegionWeight, 0.4)),
    max_results: 150,
  };
  return {
    item_id: `cte-${item.id}`,
    cte_id: Number(item.id),
    name: item.name,
    category: item.category || "",
    manufacturer: item.manufacturer || "",
    quantity: 1,
    region: request.region,
    months_back: request.months_back,
    settings_mode: request.settings_mode,
    time_importance_label: request.time_importance_label,
    same_region_importance_label: request.same_region_importance_label,
    other_region_importance_label: request.other_region_importance_label,
    time_weight_mode: request.time_weight_mode,
    time_decay: request.time_decay,
    same_region_weight: request.same_region_weight,
    other_region_weight: request.other_region_weight,
    max_results: request.max_results,
    excluded_ids: [],
    price_overrides: [],
    weight_overrides: [],
    manual_entries: [],
  };
}

function refreshVisibleSearchResults() {
  if (!state.searchResultItems.length) {
    return;
  }
  renderSearchResults(normalizeQuery(els.queryInput.value), {
    items: state.searchResultItems,
    alternatives: state.hintAlternatives,
  });
}

function createManualBatchItem() {
  const lotSettings = collectBatchSettings();
  return {
    item_id: `manual-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    cte_id: 0,
    name: "",
    category: "",
    manufacturer: "",
    quantity: 1,
    region: lotSettings.region,
    months_back: Number(pickValue(lotSettings.monthsBack, 1)),
    settings_mode: lotSettings.mode,
    time_importance_label: lotSettings.timeImportanceLabel,
    same_region_importance_label: lotSettings.sameRegionImportanceLabel,
    other_region_importance_label: lotSettings.otherRegionImportanceLabel,
    time_weight_mode: "linear_floor",
    time_decay: Number(pickValue(lotSettings.timeDecay, 0.65)),
    same_region_weight: Number(pickValue(lotSettings.sameRegionWeight, 1)),
    other_region_weight: Number(pickValue(lotSettings.otherRegionWeight, 0.4)),
    max_results: 150,
    excluded_ids: [],
    price_overrides: [],
    weight_overrides: [],
    manual_entries: [{
      label: "",
      region: lotSettings.region || "",
      supplier_region: "",
      unit_price: 0,
      vat_percent: null,
      weight_multiplier: 1,
      similarity: 1,
    }],
  };
}

function addManualBatchItem() {
  const manualItem = createManualBatchItem();
  state.batchItems.push(manualItem);
  state.batchPanelVisible = true;
  invalidateBatchResults();
  renderBatchPanel();
  syncActiveLot();
  setStatus("В лот добавлена пользовательская позиция. Заполни название, цену и НДС.", "success");
}

function upsertBatchItem(batchItem, options = {}) {
  const { silent = true } = options;
  const manualOnly = isManualBatchItem(batchItem);
  const index = state.batchItems.findIndex((entry) => {
    if (manualOnly || isManualBatchItem(entry)) {
      return entry.item_id === batchItem.item_id;
    }
    return Number(entry.cte_id) === Number(batchItem.cte_id);
  });
  if (index >= 0) {
    state.batchItems[index] = { ...state.batchItems[index], ...batchItem };
  } else {
    state.batchItems.push(batchItem);
  }
  state.batchPanelVisible = true;
  invalidateBatchResults();
  renderBatchPanel();
  refreshVisibleSearchResults();
  syncActiveLot();
  if (!silent) {
    setStatus(`Позиция СТЕ ${batchItem.cte_id} добавлена в лот`, "success");
  }
}

function removeBatchItemByID(itemID, options = {}) {
  const { silent = true } = options;
  const before = state.batchItems.length;
  state.batchItems = state.batchItems.filter((item) => item.item_id !== itemID);
  if (state.batchItems.length === before) {
    return;
  }
  if (!state.batchItems.length) {
    state.batchPanelVisible = false;
  }
  invalidateBatchResults();
  renderBatchPanel();
  refreshVisibleSearchResults();
  syncActiveLot();
  if (!silent) {
    setStatus("Позиция удалена из лота", "success");
  }
}

function removeBatchItemByCTE(cteID, options = {}) {
  const { silent = true } = options;
  const before = state.batchItems.length;
  state.batchItems = state.batchItems.filter((item) => Number(item.cte_id) !== Number(cteID));
  if (state.batchItems.length === before) {
    return;
  }
  if (!state.batchItems.length) {
    state.batchPanelVisible = false;
  }
  invalidateBatchResults();
  renderBatchPanel();
  refreshVisibleSearchResults();
  syncActiveLot();
  if (!silent) {
    setStatus(`Позиция СТЕ ${cteID} удалена из лота`, "success");
  }
}

function addCheckedSearchResultsToBatch() {
  const ids = Array.from(state.searchSelectionIDs);
  if (!ids.length) {
    setStatus("Отметь хотя бы один вариант из полного списка", "warning");
    return;
  }

  let added = 0;
  ids.forEach((id) => {
    const item = state.searchResultItems.find((entry) => Number(entry.id) === Number(id));
    if (!item) {
      return;
    }
    const batchItem = buildBatchItemFromSearchItem(item);
    upsertBatchItem(batchItem, { silent: true });
    added += 1;
  });

  setStatus(`В лот добавлено или обновлено ${formatInt(added)} позиций из результатов поиска`, "success");
}

function updateSearchSelectionAction() {
  if (!els.addSearchSelectionBtn) {
    return;
  }
  els.addSearchSelectionBtn.classList.add("hidden");
}

function upsertCurrentSelectionInBatch(silent = false) {
  const item = snapshotCurrentSelectionForBatch();
  if (!item) {
    setStatus("Сначала выбери товар, потом добавляй его в лот", "warning");
    return;
  }

  const index = state.batchItems.findIndex((entry) => Number(entry.cte_id) === Number(item.cte_id));
  if (index >= 0) {
    state.batchItems[index] = item;
  } else {
    state.batchItems.push(item);
  }
  invalidateBatchResults();
  renderBatchPanel();
  refreshVisibleSearchResults();
  syncActiveLot();
  if (!silent) {
    setStatus(`Товар с кодом СТЕ ${item.cte_id} сохранен в лот`, "success");
  }
}

function syncSelectedItemIntoBatchIfPresent() {
  if (!state.selected) {
    return;
  }
  const exists = state.batchItems.some((entry) => Number(entry.cte_id) === Number(state.selected.id));
  if (!exists) {
    return;
  }
  upsertCurrentSelectionInBatch(true);
}

function buildBatchRequest() {
  const lotSettings = collectBatchSettings();
  return {
    batch_name: String(els.batchName.value || "").trim() || "Лот НМЦК",
    region: lotSettings.region,
    months_back: Number(pickValue(lotSettings.monthsBack, 1)),
    settings_mode: lotSettings.mode,
    time_importance_label: lotSettings.timeImportanceLabel,
    same_region_importance_label: lotSettings.sameRegionImportanceLabel,
    other_region_importance_label: lotSettings.otherRegionImportanceLabel,
    time_weight_mode: "linear_floor",
    time_decay: Number(pickValue(lotSettings.timeDecay, 0.65)),
    same_region_weight: Number(pickValue(lotSettings.sameRegionWeight, 1)),
    other_region_weight: Number(pickValue(lotSettings.otherRegionWeight, 0.4)),
    max_results: 150,
    items: state.batchItems.filter(batchItemHasData).map((item) => ({
      item_id: item.item_id,
      cte_id: Number(item.cte_id),
      name: item.name,
      category: item.category,
      manufacturer: item.manufacturer,
      quantity: parsePositiveInteger(item.quantity, 1),
      region: lotSettings.region,
      months_back: Number(pickValue(lotSettings.monthsBack, 1)),
      settings_mode: lotSettings.mode,
      time_importance_label: lotSettings.timeImportanceLabel,
      same_region_importance_label: lotSettings.sameRegionImportanceLabel,
      other_region_importance_label: lotSettings.otherRegionImportanceLabel,
      time_weight_mode: "linear_floor",
      time_decay: Number(pickValue(lotSettings.timeDecay, 0.65)),
      same_region_weight: Number(pickValue(lotSettings.sameRegionWeight, 1)),
      other_region_weight: Number(pickValue(lotSettings.otherRegionWeight, 0.4)),
      max_results: Number(item.max_results || 150),
      excluded_ids: item.excluded_ids || [],
      price_overrides: item.price_overrides || [],
      weight_overrides: item.weight_overrides || [],
      manual_entries: item.manual_entries || [],
    })),
  };
}

function createBatchRequestFromLot(lot, options = {}) {
  const { prefixLotName = false } = options;
  const settings = lot?.batchSettings || defaultLotSettings();
  const lotName = String(lot?.name || "").trim() || "Лот НМЦК";
  const items = (Array.isArray(lot?.batchItems) ? lot.batchItems : []).filter(batchItemHasData);

  return {
    batch_name: lotName,
    region: settings.region || "",
    months_back: Number(pickValue(settings.monthsBack, 1)),
    settings_mode: settings.mode || "simple",
    time_importance_label: settings.timeImportanceLabel || "важно",
    same_region_importance_label: settings.sameRegionImportanceLabel || "очень важно",
    other_region_importance_label: settings.otherRegionImportanceLabel || "средне",
    time_weight_mode: "linear_floor",
    time_decay: Number(pickValue(settings.timeDecay, 0.65)),
    same_region_weight: Number(pickValue(settings.sameRegionWeight, 1)),
    other_region_weight: Number(pickValue(settings.otherRegionWeight, 0.4)),
    max_results: 150,
    items: items.map((item) => ({
      item_id: prefixLotName ? `${lot.id}::${item.item_id}` : item.item_id,
      cte_id: Number(item.cte_id),
      name: prefixLotName ? `${lotName} / ${item.name}` : item.name,
      category: item.category,
      manufacturer: item.manufacturer,
      quantity: parsePositiveInteger(item.quantity, 1),
      region: settings.region || "",
      months_back: Number(pickValue(settings.monthsBack, 1)),
      settings_mode: settings.mode || "simple",
      time_importance_label: settings.timeImportanceLabel || "важно",
      same_region_importance_label: settings.sameRegionImportanceLabel || "очень важно",
      other_region_importance_label: settings.otherRegionImportanceLabel || "средне",
      time_weight_mode: "linear_floor",
      time_decay: Number(pickValue(settings.timeDecay, 0.65)),
      same_region_weight: Number(pickValue(settings.sameRegionWeight, 1)),
      other_region_weight: Number(pickValue(settings.otherRegionWeight, 0.4)),
      max_results: Number(item.max_results || 150),
      excluded_ids: item.excluded_ids || [],
      price_overrides: item.price_overrides || [],
      weight_overrides: item.weight_overrides || [],
      manual_entries: item.manual_entries || [],
    })),
  };
}

function getFilledLots() {
  return (state.lots || []).filter((lot) => Array.isArray(lot.batchItems) && lot.batchItems.some(batchItemHasData));
}

function buildLotsDocumentRequest() {
  const activeLot = getActiveLot();
  const filledLots = getFilledLots();
  if (filledLots.length <= 1) {
    return {
      request: buildBatchRequest(),
      lotCount: Math.max(filledLots.length, 1),
      combined: false,
    };
  }

  const activeSettings = activeLot?.batchSettings || defaultLotSettings();
  const combinedItems = filledLots.flatMap((lot) => createBatchRequestFromLot(lot, { prefixLotName: true }).items);

  return {
    combined: true,
    lotCount: filledLots.length,
    request: {
      batch_name: `Сводный документ по ${filledLots.length} лотам`,
      region: activeSettings.region || "по настройкам лотов",
      months_back: Number(pickValue(activeSettings.monthsBack, 1)),
      settings_mode: activeSettings.mode || "simple",
      time_importance_label: activeSettings.timeImportanceLabel || "важно",
      same_region_importance_label: activeSettings.sameRegionImportanceLabel || "очень важно",
      other_region_importance_label: activeSettings.otherRegionImportanceLabel || "средне",
      time_weight_mode: "linear_floor",
      time_decay: Number(pickValue(activeSettings.timeDecay, 0.65)),
      same_region_weight: Number(pickValue(activeSettings.sameRegionWeight, 1)),
      other_region_weight: Number(pickValue(activeSettings.otherRegionWeight, 0.4)),
      max_results: 150,
      items: combinedItems,
    },
  };
}

function manualEntryForBatchItem(item) {
  if (Array.isArray(item.manual_entries) && item.manual_entries.length) {
    return item.manual_entries[0];
  }
  return {
    label: "",
    region: item.region || "",
    supplier_region: "",
    unit_price: 0,
    vat_percent: null,
    weight_multiplier: 1,
    similarity: 1,
  };
}

function batchItemHasData(item) {
  if (!isManualBatchItem(item)) {
    return Number(item?.cte_id || 0) > 0;
  }
  const entry = manualEntryForBatchItem(item);
  return Number(entry.unit_price || 0) > 0;
}

function effectivePeriodBounds(summary, fallbackMonths = 1) {
  const referenceDate = summary?.reference_date || "";
  if (!referenceDate) {
    return null;
  }
  const end = new Date(referenceDate);
  if (Number.isNaN(end.getTime())) {
    return null;
  }
  const appliedMonths = Number(summary?.window_hint?.applied_months_back || fallbackMonths || 0);
  if (appliedMonths <= 0) {
    return null;
  }
  const start = new Date(end);
  start.setMonth(start.getMonth() - appliedMonths);
  return { start, end };
}

function effectivePeriodLabel(summary, fallbackMonths = 1) {
  const bounds = effectivePeriodBounds(summary, fallbackMonths);
  if (!bounds) {
    return "данные пользователя";
  }
  return `${formatDate(bounds.start)} - ${formatDate(bounds.end)}`;
}

function effectiveLotPeriodLabel(items) {
  let earliest = null;
  let latest = null;
  (items || []).forEach((item) => {
    const bounds = effectivePeriodBounds(item?.result?.summary, item?.result?.parameters?.months_back || 1);
    if (!bounds) {
      return;
    }
    if (!earliest || bounds.start < earliest) {
      earliest = bounds.start;
    }
    if (!latest || bounds.end > latest) {
      latest = bounds.end;
    }
  });
  if (!earliest || !latest) {
    return "данные пользователя";
  }
  return `${formatDate(earliest)} - ${formatDate(latest)}`;
}

function renderManualBatchEditor(item) {
  const entry = manualEntryForBatchItem(item);
  return `
    <div class="batch-manual-grid">
      <label class="field">
        <span>Название позиции</span>
        <input type="text" data-manual-item-id="${escapeHtml(item.item_id)}" data-manual-item-field="name" value="${escapeHtml(item.name || "")}" placeholder="Например: коммерческое предложение поставщика">
      </label>
      <label class="field">
        <span>Производитель</span>
        <input type="text" data-manual-item-id="${escapeHtml(item.item_id)}" data-manual-item-field="manufacturer" value="${escapeHtml(item.manufacturer || "")}" placeholder="Если известен">
      </label>
      <label class="field">
        <span>Категория</span>
        <input type="text" data-manual-item-id="${escapeHtml(item.item_id)}" data-manual-item-field="category" value="${escapeHtml(item.category || "")}" placeholder="Для удобства в документе">
      </label>
      <label class="field">
        <span>Цена с НДС</span>
        <input type="number" min="0" step="0.01" data-manual-item-id="${escapeHtml(item.item_id)}" data-manual-item-field="unit_price" value="${formatInputNumber(entry.unit_price)}" placeholder="0.00">
      </label>
      <label class="field">
        <span>НДС, %</span>
        <input type="number" min="0" max="100" step="0.01" data-manual-item-id="${escapeHtml(item.item_id)}" data-manual-item-field="vat_percent" value="${formatOptionalNumber(entry.vat_percent)}" placeholder="20">
      </label>
    </div>
  `;
}

function renderBatchPanel() {
  const shouldShow = state.batchPanelVisible && state.batchItems.length > 0;
  if (!shouldShow) {
    els.contentGrid.classList.remove("has-rail");
    els.batchPanel.classList.add("hidden");
    els.batchItems.innerHTML = "";
    els.batchResults.innerHTML = "";
    els.batchDisclosure.classList.add("hidden");
    els.batchSummary.classList.add("hidden");
    els.batchSummary.innerHTML = "";
    if (!state.batchItems.length) {
      updateBatchDocLink("");
    }
    return;
  }

  els.contentGrid.classList.add("has-rail");
  els.batchPanel.classList.remove("hidden");
  const filledLots = getFilledLots();
  els.batchDocBtn.textContent = filledLots.length > 1 ? `PDF по ${filledLots.length} лотам` : "PDF по лоту";
  els.openBatchDocLink.textContent = filledLots.length > 1 ? "Открыть PDF всех лотов" : "Открыть PDF лота";
  if (!els.batchName.value.trim()) {
    els.batchName.value = "Лот НМЦК";
  }

  els.batchItems.innerHTML = state.batchItems.map((item, index) => {
    const manualOnly = isManualBatchItem(item);
    return `
      <article class="batch-item${manualOnly ? " batch-item-manual" : ""}" data-batch-item-id="${escapeHtml(item.item_id)}">
        <div class="batch-item-head">
          <div>
            <p class="section-kicker">${manualOnly ? `Позиция пользователя ${index + 1}` : `Позиция ${index + 1}`}</p>
            <strong>${escapeHtml(item.name || (manualOnly ? "Новая пользовательская позиция" : "Без названия"))}</strong>
            <p class="muted">${manualOnly
              ? `Источник цены: добавлено пользователем${item.manufacturer ? ` | ${escapeHtml(item.manufacturer)}` : ""}`
              : `Код СТЕ ${item.cte_id} | ${escapeHtml(item.category || "без категории")} | ${escapeHtml(item.manufacturer || "производитель не указан")}`}</p>
          </div>
          <div class="batch-item-actions">
            <input class="table-input batch-qty" type="number" min="1" step="1" data-batch-qty="${escapeHtml(item.item_id)}" value="${formatIntegerInput(item.quantity || 1)}">
            <button type="button" class="button button-ghost" data-batch-remove="${escapeHtml(item.item_id)}">Удалить</button>
          </div>
        </div>
        <div class="batch-item-meta">
          <span class="row-badge">${escapeHtml(item.region || "все регионы")}</span>
          <span class="row-badge">${escapeHtml(item.settings_mode === "advanced" ? "расширенный режим" : "простой режим")}</span>
          ${manualOnly ? '<span class="row-badge row-badge-success">добавлено пользователем</span>' : ""}
        </div>
        ${manualOnly ? renderManualBatchEditor(item) : ""}
      </article>
    `;
  }).join("");

  els.batchItems.querySelectorAll("[data-batch-remove]").forEach((node) => {
    node.addEventListener("click", () => {
      removeBatchItemByID(node.getAttribute("data-batch-remove"), { silent: false });
    });
  });

  els.batchItems.querySelectorAll("[data-batch-qty]").forEach((node) => {
    node.addEventListener("change", () => {
      const itemID = node.getAttribute("data-batch-qty");
      const item = state.batchItems.find((entry) => entry.item_id === itemID);
      if (!item) {
        return;
      }
      item.quantity = parsePositiveInteger(node.value, 1);
      node.value = formatIntegerInput(item.quantity);
      invalidateBatchResults();
      renderBatchSummary();
      renderBatchResults();
      syncActiveLot();
    });
  });

  els.batchItems.querySelectorAll("[data-manual-item-id][data-manual-item-field]").forEach((node) => {
    node.addEventListener("change", () => {
      const itemID = node.getAttribute("data-manual-item-id");
      const field = node.getAttribute("data-manual-item-field");
      const item = state.batchItems.find((entry) => entry.item_id === itemID);
      if (!item || !field) {
        return;
      }
      const entry = manualEntryForBatchItem(item);
      if (!Array.isArray(item.manual_entries) || !item.manual_entries.length) {
        item.manual_entries = [entry];
      }
      switch (field) {
        case "name":
          item.name = String(node.value || "").trim();
          item.manual_entries[0].label = item.name;
          break;
        case "manufacturer":
          item.manufacturer = String(node.value || "").trim();
          break;
        case "category":
          item.category = String(node.value || "").trim();
          break;
        case "unit_price":
          item.manual_entries[0].unit_price = Number(node.value || 0);
          break;
        case "vat_percent":
          item.manual_entries[0].vat_percent = String(node.value || "").trim() === "" ? null : Number(node.value);
          break;
      }
      item.manual_entries[0].region = collectBatchSettings().region || "";
      item.region = collectBatchSettings().region || "";
      invalidateBatchResults();
      renderBatchPanel();
      syncActiveLot();
    });
  });

  renderBatchSummary();
  renderBatchResults();
}

function renderBatchSummary() {
  if (!state.lastBatchResponse) {
    els.batchSummary.classList.add("hidden");
    els.batchSummary.innerHTML = "";
    return;
  }
  const summary = state.lastBatchResponse.summary || {};
  els.batchSummary.classList.remove("hidden");
  els.batchSummary.innerHTML = `
    <div>
      <p class="section-kicker">Итог по лоту</p>
      <strong>${escapeHtml(summary.batch_name || els.batchName.value || "Лот НМЦК")}</strong>
      <p class="muted">Позиций: ${formatInt(summary.item_count)} | закупок в документе: ${formatInt(summary.total_document_contracts)} | период цен: ${escapeHtml(effectiveLotPeriodLabel(state.lastBatchResponse.items || []))}</p>
    </div>
    <div class="right">
      <p class="section-kicker">Сумма</p>
      <strong>${money(summary.grand_total)}</strong>
      <p class="muted">без НДС: ${money(summary.grand_total_no_vat)}</p>
    </div>
  `;
}

function renderBatchResults() {
  if (!state.batchPanelVisible || !state.batchItems.length) {
    els.batchResults.innerHTML = "";
    els.batchDisclosure.classList.add("hidden");
    return;
  }
  if (!state.lastBatchResponse) {
    els.batchDisclosure.classList.remove("hidden");
    els.batchResults.innerHTML = `
      <div class="batch-results-list">
        ${state.batchItems.map((item) => `
          <article class="batch-result-card compact-batch-card">
            <div class="batch-result-head">
              <div>
                <strong>${escapeHtml(item.name || (isManualBatchItem(item) ? "Новая пользовательская позиция" : "Без названия"))}</strong>
                <p class="muted">${isManualBatchItem(item)
                  ? "добавлено пользователем"
                  : `${escapeHtml(item.manufacturer || "производитель не указан")} | код СТЕ ${formatInt(item.cte_id)}`}</p>
              </div>
              <div class="right">
                <p class="section-kicker">Кол-во</p>
                <strong>${formatIntegerInput(item.quantity || 1) || "1"}</strong>
              </div>
            </div>
            <div class="batch-result-meta compact-batch-meta">
              <span class="tag">${escapeHtml(item.category || (isManualBatchItem(item) ? "позиция пользователя" : "без категории"))}</span>
              <span class="tag">${escapeHtml(item.region || "все регионы")}</span>
            </div>
          </article>
        `).join("")}
      </div>
    `;
    return;
  }
  els.batchDisclosure.classList.remove("hidden");
  const summary = state.lastBatchResponse.summary || {};
  els.batchResults.innerHTML = `
    <div class="batch-results-list">
      ${(state.lastBatchResponse.items || []).map((item) => `
        <article class="batch-result-card compact-batch-card">
          <div class="batch-result-head">
            <div>
              <strong>${escapeHtml(item.result.selected.name)}</strong>
              <p class="muted">${item.result.selected.id > 0
                ? `${escapeHtml(item.result.selected.manufacturer || "производитель не указан")} | код СТЕ ${formatInt(item.result.selected.id)}`
                : "добавлено пользователем"}</p>
            </div>
            <div class="right">
              <p class="section-kicker">Кол-во</p>
              <strong>${formatIntegerInput(item.quantity) || "1"}</strong>
            </div>
          </div>
          <div class="batch-result-meta compact-batch-meta">
            <span class="tag">${escapeHtml(item.result.selected.category || (item.result.selected.id > 0 ? "без категории" : "позиция пользователя"))}</span>
            <span class="tag">${escapeHtml(vatLabel(item.result.summary.vat))}</span>
            <span class="tag">${formatInt(item.result.summary.document_contracts)} источников в PDF</span>
            <span class="tag">${escapeHtml(effectivePeriodLabel(item.result.summary, item.result.parameters.months_back || 1))}</span>
          </div>
          <div class="batch-result-stats">
            <div>
              <span>НМЦК с НДС</span>
              <strong>${money(item.result.summary.nmck_weighted_mean)}</strong>
            </div>
            <div>
              <span>НМЦК без НДС</span>
              <strong>${money(item.result.summary.nmck_weighted_mean_no_vat)}</strong>
            </div>
            <div>
              <span>Итого с НДС</span>
              <strong>${money(item.line_total)}</strong>
            </div>
            <div>
              <span>Итого без НДС</span>
              <strong>${money(item.line_total_no_vat)}</strong>
            </div>
          </div>
        </article>
      `).join("")}
      <article class="batch-result-card compact-batch-card batch-total-card">
        <div class="batch-result-head">
          <strong>Итого по лоту</strong>
          <span class="muted">${formatInt(summary.item_count)} позиций</span>
        </div>
        <div class="batch-result-stats">
          <div>
            <span>С НДС</span>
            <strong>${money(summary.grand_total)}</strong>
          </div>
          <div>
            <span>Без НДС</span>
            <strong>${money(summary.grand_total_no_vat)}</strong>
          </div>
          <div>
            <span>Период цен</span>
            <strong>${escapeHtml(effectiveLotPeriodLabel(state.lastBatchResponse.items || []))}</strong>
          </div>
        </div>
      </article>
    </div>
  `;
}

function focusBatchPanel() {
  if (!state.batchItems.length) {
    return;
  }
  state.batchPanelVisible = true;
  renderBatchPanel();
  els.batchPanel.setAttribute("tabindex", "-1");
  els.batchPanel.scrollIntoView({ behavior: "smooth", block: "start" });
  window.setTimeout(() => {
    els.batchPanel.focus({ preventScroll: true });
  }, 120);
  els.batchPanel.classList.add("lot-card-attention");
  window.setTimeout(() => {
    els.batchPanel.classList.remove("lot-card-attention");
  }, 1800);
}

async function calculate(excluded = []) {
  if (!state.selected) {
    setStatus("Сначала выбери товар", "warning");
    return;
  }

  const request = buildRequest(excluded);
  state.lastRequest = request;
  setStatus("Считаю НМЦК и подбираю похожие закупки...", "loading");

  try {
    const response = await fetch("/api/calculate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "Ошибка расчета");
    }
    state.lastResponse = data;
    renderSummary(data);
    renderTop(data);
    renderSteps(data);
    renderResults(data);
    applyPanelVisibility();
    setStatus(`Готово. Подходящих закупок: ${formatInt(data.summary.valid_contracts)}`, "success");
  } catch (error) {
    setStatus(error.message || "Ошибка расчета", "error");
  }
}

function renderSummary(data) {
  els.summaryPanel.classList.remove("hidden");
  const cards = [
    { label: "НМЦК с НДС", value: money(data.summary.nmck_weighted_mean), note: `${data.selected.name}. Основная цена расчета уже включает НДС.`, tone: "" },
    { label: "Контрольное значение", value: money(data.summary.nmck_weighted_median), note: `Контрольная цена с НДС. Без НДС: ${money(data.summary.nmck_weighted_median_no_vat)}`, tone: "" },
    { label: "Рабочий диапазон цен", value: `${money(data.summary.price_range_min)} - ${money(data.summary.price_range_max)}`, note: "Диапазон после очистки выбросов", tone: "" },
    { label: "Период цен", value: effectivePeriodLabel(data.summary, data.parameters.months_back || 1), note: "Фактический диапазон дат, по которому система нашла закупки", tone: "" },
    { label: "НДС", value: vatLabel(data.summary.vat), note: `НМЦК без НДС: ${money(data.summary.nmck_weighted_mean_no_vat)}`, tone: "" },
    {
      label: "Подходящие закупки",
      value: `${formatInt(data.summary.valid_contracts)} из ${formatInt(data.summary.raw_contracts)}`,
      note: data.summary.region_scope || (data.summary.fallback_to_all_region ? "По выбранному региону данных не хватило, поэтому взяли все регионы" : "Учитывался выбранный регион"),
      tone: "",
    },
    { label: "Для документа", value: `${formatInt(data.summary.document_contracts)} закупок`, note: "В PDF попадут только самые релевантные позиции", tone: "" },
    {
      label: "Настройки",
      value: data.parameters.settings_mode === "advanced" ? "Расширенный режим" : "Простой режим",
      note: data.parameters.settings_mode === "advanced"
        ? `Свежесть ${formatCoefficient(data.parameters.time_decay)} | свой регион ${formatCoefficient(data.parameters.same_region_weight)} | другие регионы ${formatCoefficient(data.parameters.other_region_weight)}`
        : `Свежесть: ${data.parameters.time_importance_label}; свой регион: ${data.parameters.same_region_importance_label}; другие регионы: ${data.parameters.other_region_importance_label}`,
      tone: "",
    },
  ];

  if (Number(data.summary.manual_entries || 0) > 0 || Number(data.summary.manual_exclusions || 0) > 0) {
    cards.push({
      label: "Ручные правки",
      value: `${formatInt(data.summary.manual_entries || 0)} поз. / ${formatInt(data.summary.manual_exclusions || 0)} исключений`,
      note: "В расчете учтены пользовательские корректировки",
      tone: "",
    });
  }

  if ((data.summary.warnings || []).length) {
    cards.push({
      label: "Что стоит проверить",
      value: data.summary.warnings.join("; "),
      note: "Проверь ограничения и фильтры перед выпуском документа",
      tone: "warning",
    });
  }

  els.summaryCards.innerHTML = cards.map((card) => `
    <article class="summary-card${card.tone ? ` summary-card-${card.tone}` : ""}">
      <p class="section-kicker">${escapeHtml(card.label)}</p>
      <strong>${escapeHtml(card.value)}</strong>
      <p class="muted">${escapeHtml(card.note)}</p>
    </article>
  `).join("");
  renderWindowHint(data.summary.window_hint || {});
}

function renderWindowHint(hint) {
  if (!hint || (!hint.needs_expansion && !hint.expanded_automatically)) {
    els.windowHintPanel.classList.add("hidden");
    els.windowHintPanel.innerHTML = "";
    return;
  }
  const periodLabel = effectivePeriodLabel({
    reference_date: state.lastResponse?.summary?.reference_date || "",
    window_hint: hint,
  }, hint.applied_months_back || hint.requested_months_back || 1);
  els.windowHintPanel.classList.remove("hidden");
  els.windowHintPanel.innerHTML = `
    <div class="window-hint-copy">
      <p class="section-kicker">Период поиска цен</p>
      <h4>${escapeHtml(periodLabel)}</h4>
      <p class="muted">
        Поиск всегда стартует с окна в ${formatInt(hint.requested_months_back || 1)} мес.
        ${hint.expanded_automatically
          ? `Затем окно автоматически расширено до ${formatInt(hint.applied_months_back || hint.requested_months_back || 1)} мес., потому что в исходном окне было только ${formatInt(hint.requested_count || 0)} закупок.`
          : `В рабочем окне найдено ${formatInt(hint.applied_count || hint.requested_count || 0)} закупок.`}
      </p>
    </div>
  `;
}

function renderTop(data) {
  els.topPanel.classList.remove("hidden");
  els.topRecommendations.innerHTML = (data.top_recommendations || []).map((item) => `
    <article class="recommend-card">
      <p class="section-kicker">Код СТЕ ${item.cte_id}</p>
      <strong>${escapeHtml(item.name)}</strong>
      <p>${money(item.weighted_price)}</p>
      <p class="muted">${escapeHtml(item.manufacturer || "производитель не указан")} | ${escapeHtml(item.region || "регион не указан")}</p>
      <p class="muted">С НДС: ${money(item.weighted_price)} | без НДС: ${money(item.weighted_price_no_vat)} | НДС: ${escapeHtml(vatLabel(item.vat))}</p>
      <p class="muted">Закупок: ${formatInt(item.contract_count)} | похожесть ${formatSimilarity(item.similarity)}</p>
    </article>
  `).join("") || '<div class="empty-state">Рекомендации появятся после расчета.</div>';
}

function renderSteps(data) {
  els.stepsPanel.classList.remove("hidden");
  els.steps.innerHTML = (data.steps || []).map((step) => `
    <article class="step-card">
      <h4>${escapeHtml(step.title)}</h4>
      <p class="muted">${escapeHtml(step.details)}</p>
      <div class="metrics-row">${Object.entries(step.metrics || {}).map(([key, value]) => `<span class="metric-pill">${escapeHtml(friendlyMetricLabel(key))}: ${escapeHtml(friendlyMetricValue(key, value))}</span>`).join("")}</div>
    </article>
  `).join("") || '<div class="empty-state">Подробности расчета появятся после выполнения.</div>';
}

function renderResults(data) {
  els.resultsPanel.classList.remove("hidden");
  const excluded = new Set((state.lastRequest?.excluded_ids || []).map((id) => Number(id)));
  const priceOverrides = new Map((state.lastRequest?.price_overrides || []).map((item) => [Number(item.contract_row_id), Number(item.unit_price)]));
  const weightOverrides = new Map((state.lastRequest?.weight_overrides || []).map((item) => [Number(item.contract_row_id), Number(item.weight_multiplier)]));

  els.resultsBody.innerHTML = (data.results || []).map((item) => {
    const rowID = Number(item.id);
    const isManualOnly = rowID <= 0;
    const currentPrice = priceOverrides.has(rowID) ? priceOverrides.get(rowID) : Number(item.unit_price || 0);
    const currentWeight = weightOverrides.has(rowID) ? weightOverrides.get(rowID) : Number(item.weight_multiplier || 1);
    const source = (item.manual && isManualOnly) ? "добавлено пользователем" : (item.source_label || item.method || "-");
    const statuses = [];
    if (item.manual && isManualOnly) {
      statuses.push('<span class="row-badge row-badge-success">добавлено вручную</span>');
    }
    if (item.price_overridden && !isManualOnly) {
      statuses.push('<span class="row-badge row-badge-warning">цена изменена вручную</span>');
    }
    if (!isManualOnly && currentWeight !== 1) {
      statuses.push('<span class="row-badge">влияние изменено вручную</span>');
    }
    if (!statuses.length) {
      statuses.push('<span class="row-badge">подобрано автоматически</span>');
    }

    return `
      <tr data-tone="${item.manual ? "manual" : "auto"}">
        <td>${isManualOnly ? '<span class="row-badge">вручную</span>' : `<input type="checkbox" data-row-id="${rowID}" ${excluded.has(rowID) ? "checked" : ""}>`}</td>
        <td>${formatDate(item.contract_date)}</td>
        <td>${escapeHtml(item.cte_name)}</td>
        <td>
          <div>${money(item.original_unit_price || item.unit_price)}</div>
          ${item.price_overridden ? `<div class="muted">исходно ${money(item.original_unit_price)}</div>` : ""}
        </td>
        <td>
          <div>${escapeHtml(vatLabel(item.vat))}</div>
          <div class="muted">без НДС ${money(item.unit_price_no_vat)}</div>
        </td>
        <td>${isManualOnly ? '<span class="muted">задается в ручной позиции</span>' : `<input class="table-input" type="number" min="0" step="0.01" data-price-override-id="${rowID}" data-original-price="${Number(item.original_unit_price || item.unit_price || 0).toFixed(6)}" value="${formatInputNumber(currentPrice)}">`}</td>
        <td>${escapeHtml(item.region || item.supplier_region || "-")}</td>
        <td>${formatSimilarity(item.similarity)}</td>
        <td>${formatWeight(item.final_weight)}</td>
        <td>${isManualOnly ? '<span class="muted">задано в ручной позиции</span>' : `<input class="table-input" type="number" min="0.05" max="5" step="0.05" data-weight-override-id="${rowID}" data-base-weight="1" value="${formatInputNumber(currentWeight)}">`}</td>
        <td>${escapeHtml(source)}</td>
        <td><div class="row-badges">${statuses.join("")}</div></td>
      </tr>
    `;
  }).join("");
}

function addManualEntryRow() {
  const entries = getManualEntries();
  entries.push({
    label: "",
    region: els.regionSelect.value || "",
    supplier_region: "",
    unit_price: "",
    vat_percent: "",
    weight_multiplier: 1,
    similarity: 1,
  });
  renderManualEntries(entries);
}

function renderManualEntries(entries) {
  if (!state.selected) {
    els.manualPanel.classList.add("hidden");
    return;
  }

  const items = Array.isArray(entries) ? entries : [];
  els.manualPanel.classList.remove("hidden");

  if (!items.length) {
    els.manualEntries.innerHTML = '<div class="empty-state">Пока нет товаров, добавленных пользователем. Добавь свой товар, если у тебя есть надежная цена от поставщика или системе не хватает аналогов. НДС этой позиции тоже участвует в расчете.</div>';
    return;
  }

  els.manualEntries.innerHTML = items.map((entry, index) => `
    <div class="manual-entry" data-manual-entry="${index}">
      <label>
        <span class="field-label">Наименование</span>
        <input type="text" data-manual-field="label" value="${escapeHtml(entry.label || "")}" placeholder="Например: коммерческое предложение поставщика">
      </label>
      <label>
        <span class="field-label">Регион</span>
        <input type="text" data-manual-field="region" value="${escapeHtml(entry.region || "")}" placeholder="Например: регион заказчика">
      </label>
      <label>
        <span class="field-label">Регион поставщика</span>
        <input type="text" data-manual-field="supplier_region" value="${escapeHtml(entry.supplier_region || "")}" placeholder="Например: регион поставщика">
      </label>
      <label>
        <span class="field-label">Цена</span>
        <input type="number" min="0" step="0.01" data-manual-field="unit_price" value="${formatInputNumber(entry.unit_price)}" placeholder="0.00">
      </label>
      <label>
        <span class="field-label">НДС, %</span>
        <input type="number" min="0" max="100" step="0.01" data-manual-field="vat_percent" value="${formatOptionalNumber(entry.vat_percent)}" placeholder="Например: 20">
      </label>
      <label>
        <span class="field-label">Вес ручной позиции</span>
        <input type="number" min="0.05" max="5" step="0.05" data-manual-field="weight_multiplier" value="${formatInputNumber(entry.weight_multiplier || 1)}">
      </label>
      <label>
        <span class="field-label">Похожесть на товар</span>
        <input type="number" min="0.05" max="1" step="0.05" data-manual-field="similarity" value="${formatInputNumber(entry.similarity || 1)}">
      </label>
      <label>
        <span class="field-label">Подсказка</span>
        <input type="text" value="Оставь пусто, чтобы взять ставку НДС выбранного товара" disabled>
      </label>
      <div class="manual-entry-actions">
        <button type="button" class="button button-ghost" data-remove-manual-entry="${index}">Удалить</button>
      </div>
    </div>
  `).join("");

  els.manualEntries.querySelectorAll("[data-remove-manual-entry]").forEach((node) => {
    node.addEventListener("click", () => {
      const removeIndex = Number(node.getAttribute("data-remove-manual-entry"));
      const nextEntries = getManualEntries().filter((_, index) => index !== removeIndex);
      renderManualEntries(nextEntries);
    });
  });
}

function getManualEntries() {
  return Array.from(els.manualEntries.querySelectorAll("[data-manual-entry]")).map((node) => {
    const get = (field) => node.querySelector(`[data-manual-field="${field}"]`)?.value ?? "";
    return {
      label: String(get("label")).trim(),
      region: String(get("region")).trim(),
      supplier_region: String(get("supplier_region")).trim(),
      unit_price: Number(get("unit_price") || 0),
      vat_percent: String(get("vat_percent")).trim() === "" ? null : Number(get("vat_percent")),
      weight_multiplier: Number(get("weight_multiplier") || 1),
      similarity: Number(get("similarity") || 1),
    };
  }).filter((item) => item.unit_price > 0 || item.label || item.region || item.supplier_region);
}

function getPriceOverrides() {
  return Array.from(document.querySelectorAll("[data-price-override-id]")).map((node) => {
    const value = Number(node.value || 0);
    const original = Number(node.getAttribute("data-original-price") || 0);
    if (value <= 0 || Math.abs(value - original) < 0.000001) {
      return null;
    }
    return {
      contract_row_id: Number(node.getAttribute("data-price-override-id")),
      unit_price: value,
    };
  }).filter(Boolean);
}

function getWeightOverrides() {
  return Array.from(document.querySelectorAll("[data-weight-override-id]")).map((node) => {
    const value = Number(node.value || 0);
    const base = Number(node.getAttribute("data-base-weight") || 1);
    if (value <= 0 || Math.abs(value - base) < 0.000001) {
      return null;
    }
    return {
      contract_row_id: Number(node.getAttribute("data-weight-override-id")),
      weight_multiplier: value,
    };
  }).filter(Boolean);
}

function getExcludedIDs() {
  return Array.from(document.querySelectorAll("[data-row-id]:checked")).map((node) => Number(node.getAttribute("data-row-id")));
}

async function createDocument() {
  if (!state.selected) {
    setStatus("Сначала выбери товар и при необходимости выполни расчет", "warning");
    return;
  }

  state.lastRequest = buildRequest(getExcludedIDs());

  setStatus("Формирую PDF-документ...", "loading");
  try {
    const response = await fetch("/api/document", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(state.lastRequest),
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "Не удалось сформировать документ");
    }
    updateDocLink(data.file_url || "");
    setStatus(`PDF версии v${data.version} сформирован`, "success");
    await bootstrap();
  } catch (error) {
    setStatus(error.message || "Ошибка формирования PDF", "error");
  }
}

async function calculateBatch() {
  if (!state.batchItems.length) {
    setStatus("Сначала добавь хотя бы один товар в лот", "warning");
    return;
  }
  focusBatchPanel();
  const request = buildBatchRequest();
  if (!request.items.length) {
    setStatus("В лоте пока нет заполненных позиций. Добавь товар из поиска или введи свою позицию вручную.", "warning");
    return;
  }
  state.lastBatchRequest = request;
  setStatus("Считаю НМЦК по лоту...", "loading");
  try {
    const response = await fetch("/api/calculate/batch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "Ошибка расчета лота");
    }
    state.lastBatchResponse = data;
    updateBatchDocLink("");
    renderBatchPanel();
    focusBatchPanel();
    syncActiveLot();
    setStatus(`Лот рассчитан. Позиций: ${formatInt(data.summary.item_count)}, сумма ${money(data.summary.grand_total)}`, "success");
  } catch (error) {
    setStatus(error.message || "Ошибка расчета лота", "error");
  }
}

async function createBatchDocument() {
  if (!state.batchItems.length) {
    setStatus("Сначала добавь товары в лот", "warning");
    return;
  }
  focusBatchPanel();
  const docFlow = buildLotsDocumentRequest();
  const request = docFlow.request;
  if (!request.items.length) {
    setStatus("В лоте пока нет заполненных позиций для PDF.", "warning");
    return;
  }
  state.lastBatchRequest = request;
  setStatus(docFlow.combined ? `Формирую общий PDF по ${docFlow.lotCount} лотам...` : "Формирую PDF по лоту...", "loading");
  try {
    const response = await fetch("/api/document/batch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "Не удалось сформировать PDF по лоту");
    }
    updateBatchDocLink(data.file_url || "");
    focusBatchPanel();
    if (docFlow.combined) {
      state.lots = state.lots.map((lot) => (
        Array.isArray(lot.batchItems) && lot.batchItems.length
          ? { ...lot, batchDocURL: data.file_url || lot.batchDocURL || "" }
          : lot
      ));
      persistLots();
      renderLotDrafts();
    }
    setStatus(docFlow.combined ? `Сводный PDF по ${docFlow.lotCount} лотам версии v${data.version} сформирован` : `PDF по лоту версии v${data.version} сформирован`, "success");
    await bootstrap();
    updateBatchDocLink(data.file_url || "");
    syncActiveLot();
    focusBatchPanel();
  } catch (error) {
    setStatus(error.message || "Ошибка формирования PDF по лоту", "error");
  }
}

function updateDocLink(fileURL) {
  if (!fileURL) {
    els.openDocLink.classList.add("hidden");
    els.openDocLink.removeAttribute("href");
    return;
  }
  els.openDocLink.href = fileURL;
  els.openDocLink.classList.remove("hidden");
}

function updateBatchDocLink(fileURL) {
  if (!fileURL) {
    els.openBatchDocLink.classList.add("hidden");
    els.openBatchDocLink.removeAttribute("href");
    return;
  }
  els.openBatchDocLink.href = fileURL;
  els.openBatchDocLink.classList.remove("hidden");
}

function renderRecentDocuments() {
  const docs = state.recentDocuments || [];
  if (!docs.length) {
    els.recentDocs.innerHTML = '<div class="empty-state">Документы пока не формировались.</div>';
    return;
  }

  els.recentDocs.innerHTML = docs.map((doc, index) => `
    <article class="recent-doc">
      <button type="button" class="recent-doc" data-doc-index="${index}">
        <span class="recent-title">${escapeHtml(doc.name)}</span>
        <span class="recent-meta">${doc.doc_type === "batch" ? `Лот | ${formatInt(doc.item_count || 0)} поз. | v${doc.version}` : `СТЕ ${doc.cte_id} | v${doc.version}`} | ${escapeHtml(doc.region || "все регионы")}</span>
        <span class="recent-meta">${escapeHtml(doc.summary || "")}</span>
      </button>
      <div class="recent-actions">
        <span class="muted">${formatDateTime(doc.created_at)}</span>
        ${doc.file_url ? `<a class="button button-ghost" href="${escapeHtml(doc.file_url)}" target="_blank" rel="noreferrer">PDF</a>` : ""}
      </div>
    </article>
  `).join("");

  els.recentDocs.querySelectorAll("[data-doc-index]").forEach((node) => {
    node.addEventListener("click", () => {
      const doc = docs[Number(node.getAttribute("data-doc-index"))];
      if (!doc) {
        return;
      }
      applyRecentDocument(doc);
    });
  });
}

function applyRecentDocument(doc) {
  if (doc.doc_type === "batch") {
    if (doc.file_url) {
      updateBatchDocLink(doc.file_url);
      els.batchName.value = doc.name || "Лот НМЦК";
      setStatus(`Открыт документ по лоту v${doc.version}. Лот: ${doc.name}`, "success");
      window.open(doc.file_url, "_blank", "noreferrer");
      return;
    }
    setStatus("У документа по лоту нет доступного PDF-файла", "warning");
    return;
  }
  if (doc.file_url) {
    setStatus(`Открыт документ v${doc.version} для СТЕ ${doc.cte_id}`, "success");
    window.open(doc.file_url, "_blank", "noreferrer");
    return;
  }
  setStatus("У документа нет доступного PDF-файла", "warning");
}

function resetWorkspace() {
  syncActiveLot();
  const freshLot = createLotDraft();
  state.lots.push(freshLot);
  clearSearchWorkspace();
  loadLotIntoUI(freshLot, { reveal: false });
  setStatus(`Создан новый пустой лот: ${freshLot.name}`, "success");
}

function resetComputedPanels() {
  state.lastResponse = null;
  els.summaryPanel.classList.add("hidden");
  els.topPanel.classList.add("hidden");
  els.stepsPanel.classList.add("hidden");
  els.resultsPanel.classList.add("hidden");
  els.windowHintPanel.classList.add("hidden");
  els.summaryCards.innerHTML = "";
  els.topRecommendations.innerHTML = "";
  els.steps.innerHTML = "";
  els.resultsBody.innerHTML = "";
  els.windowHintPanel.innerHTML = "";
  applyPanelVisibility();
}

function clearStatus() {
  els.calcStatus.textContent = "";
  els.calcStatus.dataset.tone = "neutral";
  els.calcStatus.classList.add("hidden");
}

function setStatus(message, tone = "neutral") {
  if (!message) {
    clearStatus();
    return;
  }
  els.calcStatus.textContent = message;
  els.calcStatus.dataset.tone = tone;
  els.calcStatus.classList.remove("hidden");
}

async function fetchJSON(url, signal) {
  const response = await fetch(url, { signal });
  const data = await response.json();
  if (!response.ok) {
    throw new Error(data.error || `HTTP ${response.status}`);
  }
  return data;
}

function normalizeQuery(value) {
  return String(value || "").trim().toLowerCase().replace(/\s+/g, " ");
}

function formatInt(value) {
  return new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(Number(value || 0));
}

function money(value) {
  return new Intl.NumberFormat("ru-RU", { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(Number(value || 0));
}

function formatConfidence(value) {
  return `${Math.round(Number(value || 0) * 100)}%`;
}

function formatSimilarity(value) {
  return `${Math.round(Number(value || 0) * 100)}%`;
}

function formatWeight(value) {
  return Number(value || 0).toFixed(3);
}

function formatCoefficient(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return "0.00";
  }
  return numeric.toFixed(2);
}

function friendlySearchReason(reason) {
  const text = String(reason || "").trim();
  if (!text) {
    return "совпадение найдено";
  }

  const tokenMatch = text.match(/^(\d+)\/(\d+) query tokens matched in name$/i);
  if (tokenMatch) {
    return `в названии совпало ${tokenMatch[1]} из ${tokenMatch[2]} слов`;
  }

  const dictionary = {
    "fuzzy match": "похоже по написанию",
    "exact cte id match": "точное совпадение по коду СТЕ",
    "exact name match": "точное совпадение по названию",
    "name prefix match": "совпадает начало названия",
    "full phrase in name": "все словосочетание найдено в названии",
    "substring in name": "часть запроса найдена в названии",
    "name token prefix match": "совпадает начало одного из слов в названии",
    "name token partial match": "часть слова найдена в названии",
    "manufacturer match": "совпадение по производителю",
    "category fallback": "подобрано по категории",
  };

  return dictionary[text.toLowerCase()] || text;
}

function friendlyMetricLabel(key) {
  const dictionary = {
    selected_cte: "Выбранный код СТЕ",
    candidate_cte: "Найдено похожих позиций",
    relevant_cte: "Оставили подходящих позиций",
    raw_contracts: "Найдено закупок до очистки",
    window_start: "Начало периода поиска",
    reference_date: "Дата расчета",
    region_fallback: "Пришлось брать все регионы",
    lower_bound: "Нижняя граница нормальной цены",
    upper_bound: "Верхняя граница нормальной цены",
    outliers: "Убрано выбросов",
    time_decay: "Скорость устаревания цен",
    same_region_weight: "Важность своего региона",
    other_region_weight: "Учет других регионов",
    manual_adjustments: "Ручных правок",
    weighted_mean: "Основная расчетная цена",
    weighted_median: "Контрольная цена",
    price_range: "Рабочий диапазон цен",
    document_contracts: "Закупок в документе",
    coverage_target: "Целевое покрытие вклада",
    document_limit: "Лимит закупок в документе",
    region_scope: "Как выбрали регионы",
    region_tiers: "Какие уровни регионов учтены",
    requested_count: "Закупок в текущем окне",
    min_required: "Минимум для устойчивого расчета",
  };
  return dictionary[key] || key;
}

function friendlyMetricValue(key, value) {
  if (key === "region_fallback") {
    return String(value) === "true" ? "да" : "нет";
  }
  if (key === "requested_count" || key === "min_required") {
    return formatInt(value);
  }
  return value;
}

function formatInputNumber(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) {
    return "";
  }
  return String(Math.round(numeric * 1000000) / 1000000);
}

function parsePositiveInteger(value, fallback = 1) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) {
    return fallback;
  }
  return Math.max(1, Math.round(numeric));
}

function formatIntegerInput(value) {
  return String(parsePositiveInteger(value, 1));
}

function formatOptionalNumber(value) {
  if (value === null || value === undefined || value === "") {
    return "";
  }
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return "";
  }
  return String(Math.round(numeric * 1000000) / 1000000);
}

function confidenceTooltip(item) {
  const label = item.confidence_label || "Оценка совпадения";
  const explanation = item.confidence_explanation || "Совпадение рассчитано по словам в названии, производителю и отрыву от следующего варианта.";
  return `${label}. ${explanation}`;
}

function vatLabel(vat) {
  if (!vat || !vat.label) {
    return "не указана";
  }
  return vat.label;
}

function formatDate(value) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleDateString("ru-RU");
}

function formatDateTime(value) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
