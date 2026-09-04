/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// The nodeTable widget: one section per role with the first-IP helper and the role override,
// one row per node with hostname, MAC, IP and a per-node override.
//
// Widget state, stored under the step scope:
//   { rows: [{ role, group, index, hostname, macAddress, ip, overrides }],
//     roleOverrides: { [roleKey]: { enabled, values } }, firstIp: { [roleKey]: "" } }

import { t } from "./i18n.js";
import { fillIps, reconcile, resolve, roleKey, roles } from "./nodes.js";
import { button, collectFields, el, initScope, renderFields } from "./renderer.js";

/**
 * The columns a wizard can ask for. `config.columns` picks and orders them; the default is the
 * Immutable set. A provider that boots without PXE (OnPremises) leaves out `macAddress`, and the
 * widget then neither shows the column nor counts it as something still to fill.
 */
const COLUMNS = { hostname: "nodes.hostname", macAddress: "nodes.mac", ip: "nodes.ip" };
const DEFAULT_COLUMNS = ["hostname", "macAddress", "ip"];
const PLACEHOLDERS = { macAddress: "52:54:00:00:00:01" };

/**
 * The suffix the generated hostnames carry, from `config.domainFrom` as a `step.field` path.
 *
 * Without it the names stay short, which is what a provider wants when it composes the fully
 * qualified name itself: OnPremises appends `spec.kubernetes.dnsZone` to the name of every host,
 * so its node entries hold `cp1`, while an Immutable node holds `cp1.k8s.example.com`.
 */
export function domainOf(field, root) {
  const path = field.config?.domainFrom;
  if (!path) return "";

  const [step, name] = path.split(".");

  return root?.[step]?.[name] ?? "";
}

export function columnsOf(field) {
  const asked = (field.config?.columns ?? "")
    .split(",")
    .map((c) => c.trim())
    .filter((c) => c in COLUMNS);

  return asked.length ? asked : DEFAULT_COLUMNS;
}

export function makeNodeTable(wizardSteps) {
  const defaultsFields = (field) => wizardSteps.find((s) => s.id === field.config.defaultsStep).fields;

  // The rows exist as soon as the topology says so, even before this step is ever shown: the
  // preview and the "still to fill" count both need them.
  const ensure = (field, state, root) => {
    state.rows ??= [];
    state.roleOverrides ??= {};
    state.firstIp ??= {};
    Object.assign(state, reconcile(state, root[field.config.topologyStep], domainOf(field, root)));

    return state;
  };

  const widget = {
    render(container, field, state, root, onChange) {
      const topology = root[field.config.topologyStep];
      ensure(field, state, root);

      const fields = defaultsFields(field);
      const defaultsScope = root[field.config.defaultsStep];
      const rerender = () => widget.render(container, field, state, root, onChange);
      const both = () => {
        onChange();
        rerender();
      };
      const box = el("div", "nodetable");

      for (const r of roles(topology)) {
        const key = roleKey(r);
        box.append(roleHead(r, key, state, defaultsScope, both));

        const ov = state.roleOverrides[key];
        if (ov.enabled) {
          const ovBox = el("div", "list-item");
          initScope(fields, ov.values);
          renderFields(ovBox, fields, ov.values, root, onChange);
          box.append(ovBox);
        }

        box.append(rowsTable(state, key, ov, defaultsScope, fields, root, onChange, columnsOf(field)));
      }
      container.replaceChildren(box);
    },

    // MAC addresses cannot be derived, and a static node needs its IP: those are the blanks
    // that keep a configuration from being written.
    missing(field, state, root) {
      ensure(field, state, root);

      const defaultsScope = root[field.config.defaultsStep];
      const columns = columnsOf(field);
      const out = [];
      for (const row of state.rows ?? []) {
        const ov = state.roleOverrides?.[roleKey(row)];
        const mode = row.overrides.networkMode ?? (ov?.enabled ? ov.values.networkMode : undefined) ?? defaultsScope.networkMode;
        if (columns.includes("macAddress") && !row.macAddress) out.push(t("nodes.missingMac", { host: row.hostname }));
        if (columns.includes("ip") && mode !== "dhcp" && !row.ip) out.push(t("nodes.missingIp", { host: row.hostname }));
      }
      return out;
    },

    collect(field, state, root) {
      ensure(field, state, root);

      const fields = defaultsFields(field);
      const defaults = collectFields(fields, root[field.config.defaultsStep], root);
      // Overrides go through the same `when` rules as the defaults, so a hidden field in an
      // override never leaks into the file.
      return resolve(state, defaults).map((n) => ({
        ...collectFields(fields, n, root),
        hostname: n.hostname,
        macAddress: n.macAddress,
        role: n.role,
        group: n.group,
        ip: n.ip,
      }));
    },
  };

  return widget;
}

function roleHead(r, key, state, defaultsScope, both) {
  const head = el("div", "role-head");
  const title = el("h3");
  title.textContent = r.role === "custom" ? r.group : t(`nodes.role.${r.role}`);
  head.append(title);

  if (defaultsScope.networkMode !== "dhcp") {
    const first = el("input", "wz-input");
    first.placeholder = t("nodes.firstIp");
    first.value = state.firstIp[key] ?? "";
    first.addEventListener("input", () => {
      state.firstIp[key] = first.value;
    });
    head.append(
      first,
      button(t("nodes.fill"), "btn ghost small", () => {
        fillIps(state, key);
        both();
      }),
    );
  }

  const ov = (state.roleOverrides[key] ??= { enabled: false, values: {} });
  const toggle = el("label", "wz-check");
  const cb = el("input");
  cb.type = "checkbox";
  cb.checked = ov.enabled;
  cb.addEventListener("change", () => {
    ov.enabled = cb.checked;
    both();
  });
  toggle.append(cb, document.createTextNode(t("nodes.override")));
  head.append(toggle);
  return head;
}

function rowsTable(state, key, ov, defaultsScope, fields, root, onChange, columns) {
  const table = el("table");
  const thead = el("thead");
  const hr = el("tr");
  for (const h of [...columns.map((c) => COLUMNS[c]), ""]) {
    const th = el("th");
    th.textContent = h ? t(h) : "";
    hr.append(th);
  }
  thead.append(hr);
  table.append(thead);

  const tbody = el("tbody");
  for (const row of state.rows.filter((x) => roleKey(x) === key)) {
    const tr = el("tr");
    const mode = row.overrides.networkMode ?? (ov.enabled ? ov.values.networkMode : undefined) ?? defaultsScope.networkMode;
    for (const column of columns) {
      // An address the provider gets from DHCP is not a field: it says so and asks nothing.
      if (column === "ip" && mode === "dhcp") tr.append(Object.assign(el("td"), { textContent: t("nodes.dhcp") }));
      else tr.append(cell(row, column, onChange, PLACEHOLDERS[column]));
    }

    // The override opens in its own full-width row under the node, not squeezed in a cell.
    const td = el("td", "override-cell");
    const toggle = button(t("nodes.rowOverride"), "btn ghost small", () => {
      overrideRow.hidden = !overrideRow.hidden;
      toggle.setAttribute("aria-expanded", String(!overrideRow.hidden));
      if (!overrideRow.hidden && !body.hasChildNodes()) renderFields(body, fields, row.overrides, root, onChange);
    });
    toggle.setAttribute("aria-expanded", "false");
    td.append(toggle);
    tr.append(td);
    tbody.append(tr);

    const overrideRow = el("tr", "override-row");
    overrideRow.hidden = true;
    const overrideCell = el("td");
    overrideCell.colSpan = columns.length + 1;
    const body = el("div", "list-item");
    overrideCell.append(body);
    overrideRow.append(overrideCell);
    tbody.append(overrideRow);
  }
  table.append(tbody);
  return table;
}

function cell(row, key, onChange, placeholder = "") {
  const td = el("td");
  const input = el("input", "wz-input");
  input.value = row[key] ?? "";
  input.placeholder = placeholder;
  input.addEventListener("input", () => {
    row[key] = input.value;
    onChange();
  });
  td.append(input);
  return td;
}
