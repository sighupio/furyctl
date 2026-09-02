/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// Pure node logic: which nodes exist, what they are called, how their config is resolved.
// No DOM here so it can be tested with bun.

const PREFIX = { cp: "cp", etcd: "etcd", lb: "lb", infra: "infra", workers: "workers" };

export function roles(topology) {
  const out = [];
  if (topology.lbMode === "dedicated") out.push({ role: "lb", group: "", count: Number(topology.lbCount) || 1 });
  out.push({ role: "cp", group: "", count: Number(topology.cpCount) || 1 });
  if (topology.dedicatedEtcd) out.push({ role: "etcd", group: "", count: 3 });
  if (Number(topology.infraCount) > 0) out.push({ role: "infra", group: "infra", count: Number(topology.infraCount) });
  if (Number(topology.workersCount) > 0) {
    out.push({ role: "workers", group: "workers", count: Number(topology.workersCount) });
  }
  for (const g of topology.extraGroups ?? []) {
    if (g.name) out.push({ role: "custom", group: g.name, count: Number(g.count) || 1 });
  }
  return out;
}

export function roleKey(r) {
  return r.group || r.role;
}

export function hostnameFor(role, group, i, domain) {
  const prefix = role === "custom" ? group : PREFIX[role];
  return domain ? `${prefix}${i}.${domain}` : `${prefix}${i}`;
}

export function incrementIp(ip, n) {
  const parts = String(ip).split(".");
  if (parts.length !== 4 || parts.some((p) => !/^\d{1,3}$/.test(p) || Number(p) > 255)) return "";
  let num = parts.reduce((acc, p) => acc * 256 + Number(p), 0) + n;
  if (num < 0 || num > 0xffffffff) return "";
  const out = [];
  for (let i = 0; i < 4; i++) {
    out.unshift(num % 256);
    num = Math.floor(num / 256);
  }
  return out.join(".");
}

// Makes rows match the topology: keeps rows the user already edited (matched by role, group
// and index), adds missing ones with prefilled hostname and IP, drops the surplus.
export function reconcile(state, topology, domain) {
  const rows = [];
  for (const r of roles(topology)) {
    const first = state.firstIp?.[roleKey(r)] ?? "";
    for (let i = 1; i <= r.count; i++) {
      const want = hostnameFor(r.role, r.group, i, domain);
      const existing = state.rows.find((x) => x.role === r.role && x.group === r.group && x.index === i);
      const row = existing ?? {
        role: r.role,
        group: r.group,
        index: i,
        hostname: want,
        generated: want,
        macAddress: "",
        ip: "",
        overrides: {},
      };
      // The domain can change after the rows exist. A hostname nobody edited follows it; one that
      // was typed over stays as it was typed.
      if (row.hostname === row.generated) row.hostname = want;
      row.generated = want;
      if (!row.ip && first) row.ip = incrementIp(first, i - 1);
      rows.push(row);
    }
  }
  return { ...state, rows };
}

// Refills every IP of a role from its first IP, overwriting what is there.
export function fillIps(state, key) {
  const first = state.firstIp?.[key] ?? "";
  for (const row of state.rows) {
    if (roleKey(row) === key) row.ip = incrementIp(first, row.index - 1);
  }
  return state;
}

export function merge(...objects) {
  const out = {};
  for (const o of objects) {
    for (const [k, v] of Object.entries(o ?? {})) {
      const isObj = (x) => x && typeof x === "object" && !Array.isArray(x);
      out[k] = isObj(v) && isObj(out[k]) ? merge(out[k], v) : structuredClone(v);
    }
  }
  return out;
}

// The array the template reads: identity fields plus the merged node configuration.
export function resolve(state, defaults) {
  return state.rows.map((row) => {
    const ro = state.roleOverrides?.[roleKey(row)];
    const cfg = merge(defaults, ro?.enabled ? ro.values : {}, row.overrides);
    return { hostname: row.hostname, macAddress: row.macAddress, role: row.role, group: row.group, ip: row.ip, ...cfg };
  });
}
