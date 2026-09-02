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

export function makeNodeTable(wizardSteps) {
  const defaultsFields = (field) => wizardSteps.find((s) => s.id === field.config.defaultsStep).fields;

  const widget = {
    render(container, field, state, root, onChange) {
      const topology = root[field.config.topologyStep];
      const domain = root[field.config.clusterStep]?.domain ?? "";
      state.rows ??= [];
      state.roleOverrides ??= {};
      state.firstIp ??= {};
      Object.assign(state, reconcile(state, topology, domain));

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

        box.append(rowsTable(state, key, ov, defaultsScope, fields, root, onChange));
      }
      container.replaceChildren(box);
    },

    collect(field, state, root) {
      // The preview may run before this step is ever shown: generate the rows from the topology.
      state.rows ??= [];
      state.roleOverrides ??= {};
      state.firstIp ??= {};
      Object.assign(state, reconcile(state, root[field.config.topologyStep], root[field.config.clusterStep]?.domain ?? ""));

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

function rowsTable(state, key, ov, defaultsScope, fields, root, onChange) {
  const table = el("table");
  const thead = el("thead");
  const hr = el("tr");
  for (const h of ["nodes.hostname", "nodes.mac", "nodes.ip", ""]) {
    const th = el("th");
    th.textContent = h ? t(h) : "";
    hr.append(th);
  }
  thead.append(hr);
  table.append(thead);

  const tbody = el("tbody");
  for (const row of state.rows.filter((x) => roleKey(x) === key)) {
    const tr = el("tr");
    tr.append(cell(row, "hostname", onChange), cell(row, "macAddress", onChange, "52:54:00:00:00:01"));
    const mode = row.overrides.networkMode ?? (ov.enabled ? ov.values.networkMode : undefined) ?? defaultsScope.networkMode;
    tr.append(mode === "dhcp" ? Object.assign(el("td"), { textContent: t("nodes.dhcp") }) : cell(row, "ip", onChange));

    const td = el("td");
    const details = el("details", "group");
    const summary = el("summary");
    summary.textContent = t("nodes.rowOverride");
    details.append(summary);
    const body = el("div");
    details.addEventListener("toggle", () => {
      if (details.open && !body.hasChildNodes()) renderFields(body, fields, row.overrides, root, onChange);
    });
    details.append(body);
    td.append(details);
    tr.append(td);
    tbody.append(tr);
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
