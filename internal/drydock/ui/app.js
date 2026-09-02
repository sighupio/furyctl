/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// The wizard application: kind/version picker, vertical stepper, YAML drawer, review and write.

import { LANGS, getLang, setLang, t, text } from "./i18n.js";
import { makeNodeTable } from "./nodetable.js";
import { button, collectStep, el, initScope, renderFields } from "./renderer.js";
import { collectReferences } from "./sources.js";

const $ = (id) => document.getElementById(id);
const state = {
  wizards: [],
  wizard: null,
  version: "",
  answers: {},
  step: 0,
  maxStep: 0, // Furthest step reached: later ones stay locked in the stepper.
  widgets: {},
  lastPreview: null,
  written: null,
  drawerOpen: false,
};

// --- chrome -------------------------------------------------------------------
function applyStaticText() {
  for (const node of document.querySelectorAll("[data-t]")) node.textContent = t(node.dataset.t);
  $("back").textContent = t("nav.back");
  $("next").textContent = state.wizard && state.step === state.wizard.steps.length - 1 ? t("nav.review") : t("nav.next");
  $("toggle-yaml").textContent = state.drawerOpen ? t("yaml.hide") : t("yaml.show");
  document.documentElement.lang = getLang();
}

function setupLang() {
  const sel = $("lang");
  for (const code of LANGS) {
    const o = el("option");
    o.value = code;
    o.textContent = code.toUpperCase();
    o.selected = code === getLang();
    sel.append(o);
  }
  sel.addEventListener("change", () => {
    setLang(sel.value);
    applyStaticText();
    if (state.wizard) rerender();
    else renderPicker();
  });
}

function setupAppearance() {
  const btn = $("appearance");
  const KEY = "drydock.appearance";
  const read = () => {
    try {
      return localStorage.getItem(KEY) || "auto";
    } catch {
      return "auto";
    }
  };
  const paint = () => {
    const a = read();
    btn.textContent = a === "auto" ? "◐" : a === "dark" ? "●" : "○";
    btn.title = a;
    const dark = a === "dark" || (a === "auto" && window.matchMedia("(prefers-color-scheme: dark)").matches);
    document.documentElement.classList.toggle("dark", dark);
  };
  btn.addEventListener("click", () => {
    const order = ["auto", "light", "dark"];
    try {
      localStorage.setItem(KEY, order[(order.indexOf(read()) + 1) % 3]);
    } catch {}
    paint();
  });
  paint();
}

function setupDrawer() {
  const toggle = (open) => {
    state.drawerOpen = open;
    $("drawer").hidden = !open;
    applyStaticText();
  };
  $("toggle-yaml").addEventListener("click", () => toggle(!state.drawerOpen));
  $("close-yaml").addEventListener("click", () => toggle(false));
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && state.drawerOpen) toggle(false);
  });
}

function showError(message) {
  const p = $("error");
  p.hidden = !message;
  p.textContent = message ? t("error.generic", { message }) : "";
}

// 409 (output exists) and 422 (schema errors) carry a body the caller handles; everything else is an error.
async function api(path, body) {
  const init =
    body === undefined ? {} : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) };
  const res = await fetch(path, init);
  const data = await res.json().catch(() => ({}));
  if (!res.ok && res.status !== 422 && res.status !== 409) throw new Error(data.error ?? `${res.status} ${path}`);
  return { status: res.status, data };
}

// --- picker -------------------------------------------------------------------
function renderPicker() {
  const box = $("picker");
  box.replaceChildren();
  box.append(Object.assign(el("h2"), { textContent: t("app.title") }));
  box.append(Object.assign(el("p", "lead"), { textContent: t("app.subtitle") }));
  const kindSel = el("select", "wz-input wz-select");
  for (const w of state.wizards) {
    const o = el("option");
    o.value = w.kind;
    o.textContent = w.kind;
    kindSel.append(o);
  }
  const version = el("select", "wz-input wz-select");
  const help = el("span", "help");
  const syncVersion = () => {
    const w = state.wizards.find((x) => x.kind === kindSel.value);
    if (!w) return;
    version.replaceChildren();
    for (const v of w.versions) {
      const o = el("option");
      o.value = v;
      o.textContent = v === w.defaultVersion ? `${v} (default)` : v;
      o.selected = v === w.defaultVersion;
      version.append(o);
    }
    if (!w.versions.length) version.append(Object.assign(el("option"), { textContent: t("picker.noVersions"), disabled: true }));
    help.textContent = t("picker.versionHelp", { range: w.range });
  };
  kindSel.addEventListener("change", syncVersion);
  syncVersion();
  const start = button(t("picker.start"), "btn primary", async () => {
    start.disabled = true;
    start.textContent = t("picker.loading");
    try {
      const { data } = await api("/api/session", { kind: kindSel.value, version: version.value.trim() });
      startWizard(data, version.value.trim());
    } catch (e) {
      showError(e.message);
      start.disabled = false;
      start.textContent = t("picker.start");
    }
  });
  box.append(field(t("picker.kind"), kindSel), field(t("picker.version"), version), help, start);
}

function field(label, control) {
  const wrap = el("div", "wz-field");
  const l = el("label");
  l.textContent = label;
  wrap.append(l, control);
  return wrap;
}

// --- wizard -------------------------------------------------------------------
function startWizard(wizard, version) {
  state.wizard = wizard;
  state.version = version;
  state.answers = {};
  for (const s of wizard.steps) state.answers[s.id] = initScope(s.fields, {});
  state.widgets = { nodeTable: makeNodeTable(wizard.steps) };
  state.step = 0;
  state.maxStep = 0;
  state.written = null;
  state.lastPreview = null;
  $("picker").hidden = true;
  $("stepper").hidden = false;
  $("step-card").hidden = false;
  $("toggle-yaml").hidden = false;
  $("status").hidden = false;
  $("session-chip").hidden = false;
  $("session-chip").textContent = `${wizard.kind} · ${version}`;
  $("footer-version").textContent = `SIGHUP Distribution ${version}`;
  showError("");
  rerender();
  schedulePreview();
}

function collectAll() {
  const out = {};
  for (const s of state.wizard.steps) out[s.id] = collectStep(s, state.answers[s.id], state.answers, state.widgets);
  return out;
}

function goTo(i) {
  state.step = i;
  state.maxStep = Math.max(state.maxStep, i);
  rerender();
  window.scrollTo({ top: 0, behavior: "smooth" });
}

function renderStepper() {
  const steps = state.wizard.steps;
  const stepper = $("stepper");
  stepper.replaceChildren();
  const all = steps.concat([{ id: "__review", title: { en: t("review.title") } }]);
  all.forEach((s, i) => {
    const done = i < state.step;
    const active = i === state.step;
    const locked = i > state.maxStep;
    const item = el("button", `step-item${done ? " done" : ""}${active ? " active" : ""}${locked ? " locked" : ""}`);
    item.type = "button";
    item.disabled = locked;
    const bullet = el("span", "bullet");
    bullet.textContent = done ? "✓" : String(i + 1);
    const label = el("span", "step-label");
    label.textContent = text(s.title) || s.id;
    const sub = el("span", "step-sub");
    sub.textContent = done ? t("steps.done") : active ? t("steps.current") : t("steps.locked");
    label.append(sub);
    item.append(bullet, label);
    if (!locked) item.addEventListener("click", () => goTo(i));
    stepper.append(item);
  });
}

function rerender() {
  if (!state.wizard) return;
  const steps = state.wizard.steps;
  renderStepper();

  const body = $("step");
  body.className = "step-body";
  body.replaceChildren();
  $("back").disabled = state.step === 0;

  if (state.step === steps.length) {
    renderReview(body);
    $("next").hidden = true;
  } else {
    const s = steps[state.step];
    const h = el("h2");
    h.textContent = text(s.title) || s.id;
    body.append(h);
    if (text(s.description)) body.append(Object.assign(el("p", "desc"), { textContent: text(s.description) }));
    const fields = el("div");
    renderFields(fields, s.fields, state.answers[s.id], state.answers, schedulePreview, state.widgets);
    body.append(fields);
    $("next").hidden = false;
  }
  applyStaticText();
}

$("back").addEventListener("click", () => goTo(Math.max(0, state.step - 1)));
$("next").addEventListener("click", () => goTo(Math.min(state.wizard.steps.length, state.step + 1)));

// --- preview ------------------------------------------------------------------
let previewTimer = null;
function schedulePreview() {
  clearTimeout(previewTimer);
  setStatus("working");
  previewTimer = setTimeout(preview, 300);
}

async function preview() {
  try {
    const { data } = await api("/api/preview", { answers: collectAll() });
    state.lastPreview = data;
    $("empty").hidden = true;
    $("yaml").hidden = !data.yaml;
    $("yaml").textContent = data.yaml ?? "";
    $("template-error").hidden = !data.templateError;
    $("template-error").textContent = data.templateError ? `${t("preview.templateError")}\n\n${data.templateError}` : "";
    const errs = $("errors");
    errs.replaceChildren();
    errs.hidden = !data.errors?.length;
    for (const e of data.errors ?? []) {
      const li = el("li");
      const code = el("code");
      code.textContent = e.path || "/";
      li.append(code, document.createTextNode(` ${e.message}`));
      errs.append(li);
    }
    setStatus(data.templateError || data.errors?.length ? "errors" : "ok", data.errors?.length ?? 0);
    showError("");
    if (state.step === state.wizard.steps.length) rerender();
  } catch (e) {
    showError(e.message);
    setStatus("idle");
  }
}

function setStatus(kind, n = 0) {
  const pill = $("status");
  const cls = { ok: "ok", errors: "error", working: "working" }[kind] ?? "idle";
  pill.className = `pill pill-${cls}`;
  pill.textContent = kind === "errors" ? t("status.errors", { n }) : t(`status.${kind}`);
}

// --- review -------------------------------------------------------------------
function renderReview(body) {
  body.className = "step-body review";
  const h = el("h2");
  h.textContent = t("review.title");
  body.append(h, Object.assign(el("p", "desc"), { textContent: t("review.intro") }));

  const answers = collectAll();
  const refs = collectReferences(answers);
  for (const source of ["env", "file", "path", "http"]) {
    const items = refs.filter((r) => r.source === source);
    if (!items.length) continue;
    const h3 = el("h3");
    h3.textContent = t(`review.${source}`);
    const dl = el("dl");
    for (const r of items) {
      const dt = el("dt");
      dt.textContent = r.raw;
      const dd = el("dd");
      dd.textContent = r.path;
      dl.append(dt, dd);
    }
    body.append(h3, dl);
  }

  const todo = el("ul");
  const li = (s) => {
    const x = el("li");
    x.textContent = s;
    todo.append(x);
  };
  li(t("review.todo.pki", { pkiPath: answers.cluster?.pkiPath || "./pki" }));
  const noMac = (answers.nodes ?? []).filter((n) => !n.macAddress).map((n) => n.hostname);
  if (noMac.length) li(t("review.todo.mac", { hosts: noMac.join(", ") }));
  if (answers.modules?.auth?.provider === "sso" && !answers.modules.auth.dexConnectors) li(t("review.todo.dex"));
  li(t("review.todo.validate"));
  const h3 = el("h3");
  h3.textContent = t("review.todo");
  body.append(h3, todo);

  if (state.written) {
    body.append(Object.assign(el("p", "written"), { textContent: t("review.written", { path: state.written }) }));
    return;
  }

  const blocked = !state.lastPreview || state.lastPreview.templateError || state.lastPreview.errors?.length;
  if (blocked) body.append(Object.assign(el("p", "error"), { textContent: t("review.blocked") }));
  const write = button(t("review.write"), "btn primary", async () => {
    write.disabled = true;
    try {
      const { status, data } = await api("/api/write", { answers: collectAll() });
      if (status === 409) {
        showError(t("review.exists"));
        return;
      }
      if (status === 422) {
        state.lastPreview = data;
        rerender();
        return;
      }
      state.written = data.path;
      rerender();
    } catch (e) {
      showError(e.message);
      write.disabled = false;
    }
  });
  write.disabled = Boolean(blocked);
  body.append(write);
}

// --- boot ---------------------------------------------------------------------
setupLang();
setupAppearance();
setupDrawer();
applyStaticText();
setStatus("idle");
try {
  const { data } = await api("/api/wizards");
  state.wizards = data;
  renderPicker();
} catch (e) {
  showError(e.message);
}
