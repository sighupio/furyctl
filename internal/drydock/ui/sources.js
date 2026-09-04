/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// Value sources: how a field's value is written into furyctl.yaml. furyctl expands these
// patterns at apply time (internal/parser/config.go), on any string in the file.

export const SOURCES = ["value", "env", "file", "path", "http"];

const PATTERN = /^\{(env|file|path|https?):\/\/(.*)\}$/s;
// A URL carries its own scheme, so `{…}` on its own is how an http value looks while it is still
// being typed. Reading it back as a literal would lose the source the operator picked.
const UNFINISHED_URL = /^\{([^{}]*)\}$/s;

export function decode(value) {
  if (typeof value !== "string") return { source: "value", raw: value };
  const m = PATTERN.exec(value);
  if (m) {
    if (m[1] === "http" || m[1] === "https") return { source: "http", raw: `${m[1]}://${m[2]}` };

    return { source: m[1], raw: m[2] };
  }

  const u = UNFINISHED_URL.exec(value);
  if (u) return { source: "http", raw: u[1] };

  return { source: "value", raw: value };
}

export function encode(source, raw) {
  if (source === "value") return raw;
  if (source === "http") return `{${raw}}`;
  return `{${source}://${raw}}`;
}

export function suggestName(fieldId, clusterName) {
  const snake = (s) =>
    s
      .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
      .replace(/[^a-zA-Z0-9]+/g, "_")
      .replace(/^_+|_+$/g, "")
      .toUpperCase();
  const prefix = clusterName ? `${snake(clusterName)}_` : "";
  return `${prefix}${snake(fieldId)}`;
}

// Every non-literal value in the answers, in document order, deduplicated by source+raw.
export function collectReferences(answers) {
  const out = [];
  const seen = new Set();
  const walk = (v, path) => {
    if (typeof v === "string") {
      const d = decode(v);
      if (d.source === "value") return;
      const key = `${d.source}:${d.raw}`;
      if (seen.has(key)) return;
      seen.add(key);
      out.push({ source: d.source, raw: d.raw, path });
    } else if (Array.isArray(v)) {
      v.forEach((x, i) => walk(x, `${path}.${i}`));
    } else if (v && typeof v === "object") {
      for (const [k, x] of Object.entries(v)) walk(x, path ? `${path}.${k}` : k);
    }
  };
  walk(answers, "");
  return out;
}
