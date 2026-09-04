/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// Turns wizard fields into DOM and back into answers. One case per field type.
// State lives in `scope` (the step's answers object); rendering never keeps its own copy.
//
// Two callbacks travel down the tree: `onChange` after any edit (the app re-renders the
// preview), and `rerender` only after edits that can change which fields are visible
// (choices, checkboxes, sources, list add/remove). Typing never re-renders, so inputs keep focus.

import { t, text } from "./i18n.js";
import { SOURCES, decode, encode, suggestName } from "./sources.js";
import { evalWhen } from "./when.js";

const SOURCED = new Set(["text", "path", "cidr"]);

// Field types that help the user fill other fields and produce no value of their own.
const HELPERS = new Set(["preset", "dataDisk"]);

function defaultFor(field) {
  if (field.default !== undefined && field.default !== null) return structuredClone(field.default);
  switch (field.type) {
    case "bool":
      return false;
    case "choice":
      return field.options[0];
    case "list":
      return [];
    case "group":
    case "nodeTable":
      return {};
    case "keyValue":
      return [];
    default:
      return "";
  }
}

const key = (v) => JSON.stringify(v);

// A preset marked as a default belongs to the field that offers it: present while the field is
// visible, taken out of the file when the answer above hides it. A default the user switched
// off stays off, remembered in the field's own slot, which holds no answer of its own.
export function syncPresets(fields, scope, root) {
  for (const f of fields) {
    if (f.type === "preset") {
      const visible = evalWhen(f.when, scope, root);
      const list = (scope[f.target] ??= []);
      const off = scope[f.id]?.off ?? [];
      for (const p of f.presets) {
        if (!p.default) continue;
        const at = list.findIndex((item) => key(item) === key(p.value));
        if (visible && at < 0 && !off.includes(key(p.value))) list.push(structuredClone(p.value));
        if (!visible && at >= 0) list.splice(at, 1);
      }
    }
    if (f.type === "group") syncPresets(f.fields, (scope[f.id] ??= {}), root);
  }
  return scope;
}

// Fills scope with defaults for every field that has no value yet, recursively.
export function initScope(fields, scope) {
  for (const f of fields) {
    if (HELPERS.has(f.type)) continue;
    if (!Object.hasOwn(scope, f.id)) scope[f.id] = defaultFor(f);
    if (f.type === "group") initScope(f.fields, scope[f.id]);
    if (f.type === "list" && f.fields) for (const item of scope[f.id]) initScope(f.fields, item);
  }
  return scope;
}

// Visible fields only, recursively. Hidden fields are absent, so the template never sees them.
export function collectFields(fields, scope, root) {
  const out = {};
  for (const f of fields) {
    if (!evalWhen(f.when, scope, root)) continue;
    if (HELPERS.has(f.type)) continue; // Helpers fill other fields; they hold no answer of their own.
    const v = scope[f.id];
    if (f.type === "group") out[f.id] = collectFields(f.fields, v ?? {}, root);
    else if (f.type === "list" && f.fields) out[f.id] = (v ?? []).map((item) => collectFields(f.fields, item, root));
    // A blank row of a scalar list is a row the user opened and never filled: it is not an answer.
    else if (f.type === "list") out[f.id] = (v ?? []).filter((x) => x !== "" && x !== null && x !== undefined);
    // Pairs are the editable form; a map is what the file wants. A pair with no key is not one yet.
    else if (f.type === "keyValue") {
      out[f.id] = Object.fromEntries((v ?? []).filter((p) => p?.key).map((p) => [p.key, p.value ?? ""]));
    }
    else if (f.type === "number") out[f.id] = v === "" || v === null || v === undefined ? "" : Number(v);
    else out[f.id] = v;
  }
  return out;
}

// The visible required fields that are still empty, as labels the user can recognise.
// Hidden fields never count: a required field behind a `when` that is false is not asked.
export function missingRequired(fields, scope, root, widgets = {}) {
  const out = [];
  for (const f of fields) {
    if (!evalWhen(f.when, scope, root)) continue;
    const widget = widgets[f.type];
    if (widget?.missing) {
      out.push(...widget.missing(f, scope[f.id], root));
      continue;
    }
    const v = scope[f.id];
    if (f.type === "group") out.push(...missingRequired(f.fields, v ?? {}, root, widgets));
    else if (f.type === "list" && f.fields) {
      (v ?? []).forEach((item, i) => {
        for (const label of missingRequired(f.fields, item, root, widgets)) {
          out.push(`${text(f.label) || f.id} ${i + 1}: ${label}`);
        }
      });
    } else if (f.required && (v === "" || v === null || v === undefined)) {
      out.push(text(f.label) || f.id);
    }
  }
  return out;
}

// A step made of a single widget (nodeTable) collects to what the widget produces.
export function collectStep(step, scope, root, widgets) {
  const widget = step.fields.length === 1 && widgets[step.fields[0].type];
  if (widget) return widget.collect(step.fields[0], scope[step.fields[0].id], root);
  return collectFields(step.fields, scope, root);
}

export function renderFields(container, fields, scope, root, onChange, widgets = {}) {
  wireTips();
  const rerender = () => renderFields(container, fields, scope, root, onChange, widgets);
  const cb = { onChange, rerender, both: () => { onChange(); rerender(); } };
  container.replaceChildren();
  container.classList.add("fields");
  for (const f of fields) {
    if (!evalWhen(f.when, scope, root)) continue;
    const node = renderField(f, scope, root, cb, widgets);
    // Short scalars sit two per row; anything tall or repeating takes the full width.
    if (f.type === "list" || f.type === "group" || f.multiline || widgets[f.type]) node.classList.add("span-2");
    container.append(node);
  }
}

// One tip open at a time; a click elsewhere or Escape closes it.
function closeTips() {
  for (const box of document.querySelectorAll(".tip:not([hidden])")) box.hidden = true;
  for (const b of document.querySelectorAll('.hint[aria-expanded="true"]')) b.setAttribute("aria-expanded", "false");
}
// Registered on first render, not on import: this module is also loaded by tests with no DOM.
let tipsWired = false;
function wireTips() {
  if (tipsWired) return;
  tipsWired = true;
  document.addEventListener("click", (e) => {
    if (!e.target.closest(".tip, .hint")) closeTips();
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") closeTips();
  });
}

function renderField(f, scope, root, cb, widgets) {
  const widget = widgets[f.type];
  if (widget) {
    const box = el("div", "widget");
    // A widget keeps its own state object. Overrides are rendered without initScope, so it
    // may not exist yet.
    scope[f.id] ??= {};
    // Widgets get the step re-render too: one that writes into sibling fields has to make
    // them redraw, and one that only edits its own cells must not.
    widget.render(box, f, scope[f.id], root, cb.onChange, cb.rerender);
    return box;
  }

  if (f.type === "group") return group(f, scope, root, cb, widgets);

  const wrap = el("div", "wz-field");
  const tip = tipFor(f, () => scope[f.id], (v) => { scope[f.id] = v; cb.both(); });

  // A checkbox is its own label: the (?) sits on the same row, after it.
  if (f.type === "bool") {
    const row = el("div", "field-head bool-row");
    row.append(checkbox(f, scope, cb.both));
    if (tip) row.append(tip.button, tip.box);
    wrap.append(row);
    return wrap;
  }

  const head = el("div", "field-head");
  const label = el("label", f.required ? "required" : "");
  label.textContent = text(f.label) || f.id;
  head.append(label);
  if (tip) head.append(tip.button, tip.box);
  wrap.append(head);

  switch (f.type) {
    case "text":
    case "path":
    case "cidr":
      wrap.append(sourced(f, scope, root, cb));
      break;
    case "number":
      wrap.append(numberInput(f, scope, cb.onChange));
      break;
    case "choice":
      wrap.append(f.options.length <= 4 ? radios(f, scope, cb.both) : select(f, scope, cb.both));
      break;
    case "list":
      wrap.append(f.fields ? groupList(f, scope, root, cb, widgets) : scalarList(f, scope, root, cb));
      break;
    case "keyValue":
      wrap.append(keyValue(f, scope, cb));
      break;
    case "yaml":
      wrap.append(yamlBox(f, scope, cb));
      break;
    case "preset":
      wrap.append(presets(f, scope, cb));
      break;
    default:
      break;
  }

  return wrap;
}

// The (?) next to a label: opens a box with the explanation and a realistic example.
// "Use this example" fills the field when it is a scalar; for lists it appends one item.
function tipFor(f, get, set) {
  const help = text(f.help);
  if (!help && !f.example) return null;
  const btn = el("button", "hint");
  btn.type = "button";
  btn.textContent = "?";
  btn.title = t("tip.open");
  btn.setAttribute("aria-expanded", "false");
  const box = el("div", "tip");
  box.hidden = true;
  box.setAttribute("role", "dialog");
  const close = el("button", "tip-close");
  close.type = "button";
  close.textContent = "×";
  close.title = t("tip.close");
  close.setAttribute("aria-label", t("tip.close"));
  close.addEventListener("click", closeTips);
  box.append(close);
  if (help) box.append(Object.assign(el("p"), { textContent: help }));
  if (f.example) {
    const ex = el("div", "example");
    ex.append(Object.assign(el("span", "example-label"), { textContent: t("tip.example") }));
    ex.append(Object.assign(el("code"), { textContent: f.example }));
    const usable = ["text", "path", "cidr", "number"].includes(f.type) || (f.type === "list" && f.item);
    if (usable) {
      ex.append(
        button(t("tip.use"), "btn ghost small", () => {
          const v = f.type === "number" ? Number(f.example) : f.example;
          if (f.type === "list") set([...(get() ?? []), v]);
          else set(v);
        }),
      );
    }
    box.append(ex);
  }
  btn.addEventListener("click", () => {
    const open = box.hidden;
    closeTips();
    box.hidden = !open;
    btn.setAttribute("aria-expanded", String(open));
  });
  return { button: btn, box };
}

// Known-good entries from the documentation: a click puts one in the target list, another
// click takes it out. The chip shows what it does; the (?) of the chip explains why.
function presets(f, scope, cb) {
  const box = el("div", "presets");
  const list = (scope[f.target] ??= []);
  const memory = (scope[f.id] ??= { off: [] });
  memory.off ??= [];
  for (const p of f.presets) {
    const at = list.findIndex((item) => key(item) === key(p.value));
    const chip = el("button", `preset-chip${at >= 0 ? " on" : ""}`);
    chip.type = "button";
    chip.textContent = text(p.label);
    chip.title = text(p.help);
    chip.setAttribute("aria-pressed", String(at >= 0));
    chip.addEventListener("click", () => {
      if (at >= 0) {
        list.splice(at, 1);
        if (p.default) memory.off.push(key(p.value));
      } else {
        list.push(structuredClone(p.value));
        memory.off = memory.off.filter((k) => k !== key(p.value));
      }
      cb.both();
    });
    box.append(chip);
  }
  return box;
}

// An input plus the source dropdown. The stored value is always the encoded string.
function sourced(f, scope, root, cb, key = f.id) {
  const row = el("div", "with-source");
  const current = decode(scope[key] ?? "");
  // `suggest` in the wizard preselects the source for a value that is a secret or belongs outside
  // the file. Only while the field is still empty: once something is typed, the value decides.
  if (!current.raw && f.suggest) current.source = f.suggest;
  const input = f.multiline ? el("textarea", "wz-input wz-textarea") : el("input", "wz-input");
  if (!f.multiline) input.type = "text";
  input.value = current.raw ?? "";
  input.placeholder = placeholderFor(current.source, f);
  input.addEventListener("input", () => {
    scope[key] = encode(current.source, input.value);
    cb.onChange();
  });

  const sel = el("select", "wz-input wz-select source-select");
  for (const s of SOURCES) {
    const o = el("option");
    o.value = s;
    o.textContent = t(`source.${s}`);
    o.selected = s === current.source;
    sel.append(o);
  }
  sel.addEventListener("change", () => {
    const raw = sel.value === "env" ? suggestName(String(key), root?.cluster?.name ?? "") : "";
    scope[key] = encode(sel.value, raw);
    cb.both();
  });
  row.append(input, sel);
  return row;
}

function placeholderFor(source, f) {
  if (source === "env") return t("source.envName");
  if (source === "file" || source === "path") return t("source.filePath");
  if (source === "http") return t("source.url");
  return f.placeholder ?? "";
}

function numberInput(f, scope, onChange, key = f.id) {
  const input = el("input", "wz-input wz-number");
  input.type = "number";
  if (f.min !== undefined && f.min !== null) input.min = String(f.min);
  if (f.max !== undefined && f.max !== null) input.max = String(f.max);
  input.value = scope[key] === "" || scope[key] === undefined || scope[key] === null ? "" : String(scope[key]);
  input.placeholder = f.placeholder ?? "";
  input.addEventListener("input", () => {
    scope[key] = input.value === "" ? "" : Number(input.value);
    onChange();
  });
  return input;
}

function checkbox(f, scope, changed) {
  const label = el("label", "wz-check");
  const box = el("input");
  box.type = "checkbox";
  box.checked = Boolean(scope[f.id]);
  box.addEventListener("change", () => {
    scope[f.id] = box.checked;
    changed();
  });
  label.append(box, document.createTextNode(text(f.label) || f.id));
  return label;
}

function radios(f, scope, changed) {
  const box = el("div", "radios");
  const name = `r-${Math.random().toString(36).slice(2)}`;
  for (const opt of f.options) {
    const label = el("label");
    const r = el("input");
    r.type = "radio";
    r.name = name;
    r.checked = String(scope[f.id]) === String(opt);
    r.addEventListener("change", () => {
      scope[f.id] = opt;
      changed();
    });
    label.append(r, document.createTextNode(String(opt)));
    box.append(label);
  }
  return box;
}

function select(f, scope, changed) {
  const sel = el("select", "wz-input wz-select");
  for (const opt of f.options) {
    const o = el("option");
    o.value = String(opt);
    o.textContent = String(opt);
    o.selected = String(scope[f.id]) === String(opt);
    sel.append(o);
  }
  sel.addEventListener("change", () => {
    scope[f.id] = f.options.find((o) => String(o) === sel.value);
    changed();
  });
  return sel;
}

function scalarList(f, scope, root, cb) {
  const box = el("div", "scalar-list");
  // An override renders without initScope: read through a fallback and only write to the
  // scope when the user adds something, so an untouched override stays empty and inherits.
  const items = scope[f.id] ?? [];
  items.forEach((_, i) => {
    const row = el("div", "list-row");
    const control =
      f.item.type === "choice"
        ? select({ ...f.item, id: i }, items, cb.both)
        : f.item.type === "number"
          ? numberInput(f.item, items, cb.onChange, i)
          : sourced({ ...f.item, id: String(i) }, items, root, cb, i);
    control.classList.add("wz-grow");
    row.append(
      control,
      button(t("list.remove"), "btn ghost small", () => {
        items.splice(i, 1);
        cb.both();
      }),
    );
    box.append(row);
  });
  box.append(
    button(t("list.add"), "btn ghost small", () => {
      (scope[f.id] ??= []).push("");
      cb.both();
    }),
  );
  return box;
}

// Rows of key and value, which the file wants as a map. Labels, annotations and tags are the most
// repeated shape in the schema, and a card per entry was too much furniture for two words.
function keyValue(f, scope, cb) {
  const box = el("div", "keyvalue");
  const pairs = scope[f.id] ?? [];
  pairs.forEach((pair, i) => {
    const row = el("div", "list-row");
    row.append(
      plainInput(pair, "key", t("keyValue.key"), cb.onChange),
      plainInput(pair, "value", t("keyValue.value"), cb.onChange),
      button(t("list.remove"), "btn ghost small", () => {
        pairs.splice(i, 1);
        cb.both();
      }),
    );
    box.append(row);
  });
  box.append(
    button(t("list.add"), "btn ghost small", () => {
      (scope[f.id] ??= []).push({ key: "", value: "" });
      cb.both();
    }),
  );
  return box;
}

function plainInput(target, key, placeholder, onChange) {
  const input = el("input", "wz-input wz-grow");
  input.type = "text";
  input.value = target[key] ?? "";
  input.placeholder = placeholder;
  input.addEventListener("input", () => {
    target[key] = input.value;
    onChange();
  });
  return input;
}

// A block of YAML that goes into the file as it is written. The server parses it and reports a
// mistake against this field, so a stray indent does not surface as a broken document.
function yamlBox(f, scope, cb) {
  const area = el("textarea", "wz-input wz-textarea wz-yaml");
  area.value = scope[f.id] ?? "";
  area.placeholder = f.placeholder ?? "";
  area.spellcheck = false;
  area.addEventListener("input", () => {
    scope[f.id] = area.value;
    cb.onChange();
  });
  return area;
}

function groupList(f, scope, root, cb, widgets) {
  const box = el("div", "group-list");
  const items = scope[f.id] ?? [];
  items.forEach((item, i) => {
    const card = el("div", "list-item");
    const head = el("div", "list-item-head");
    head.append(
      Object.assign(el("strong"), { textContent: `${text(f.label) || f.id} ${i + 1}` }),
      button(t("list.remove"), "btn ghost small", () => {
        items.splice(i, 1);
        cb.both();
      }),
    );
    const body = el("div");
    initScope(f.fields, item);
    renderFields(body, f.fields, item, root, cb.onChange, widgets);
    card.append(head, body);
    box.append(card);
  });
  box.append(
    button(t("list.add"), "btn ghost small", () => {
      (scope[f.id] ??= []).push(initScope(f.fields, {}));
      cb.both();
    }),
  );
  return box;
}

function group(f, scope, root, cb, widgets) {
  const details = el("details", "group");
  details.open = !f.collapsed;
  const summary = el("summary");
  summary.textContent = text(f.label) || f.id;
  details.append(summary);
  const help = text(f.help);
  if (help) details.append(Object.assign(el("p", "help"), { textContent: help }));
  const body = el("div");
  scope[f.id] ??= {};
  initScope(f.fields, scope[f.id]);
  renderFields(body, f.fields, scope[f.id], root, cb.onChange, widgets);
  details.append(body);
  return details;
}

export function el(tag, className = "") {
  const e = document.createElement(tag);
  if (className) e.className = className;
  return e;
}

export function button(label, className, onClick) {
  const b = el("button", className);
  b.type = "button";
  b.textContent = label;
  b.addEventListener("click", onClick);
  return b;
}
