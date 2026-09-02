/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// Same grammar as internal/drydock/when.go: OR of ANDs of terms; a term is `a`, `!a`,
// `a == lit`, `a != lit`. Dotted identifiers resolve from root when the first segment names
// a step, from the current scope otherwise. Comparison is on String(value).

function lookup(path, scope, root) {
  const parts = path.split(".");
  let cur = root && Object.hasOwn(root, parts[0]) ? root : scope;
  for (const p of parts) {
    if (cur === null || typeof cur !== "object" || !Object.hasOwn(cur, p)) return [undefined, false];
    cur = cur[p];
  }
  return [cur, true];
}

function truthy(v) {
  if (v === null || v === undefined || v === false || v === "" || v === 0) return false;
  if (Array.isArray(v)) return v.length > 0;
  return true;
}

function evalTerm(term, scope, root) {
  const s = term.trim();
  for (const op of ["==", "!="]) {
    const i = s.indexOf(op);
    if (i >= 0) {
      const [v, found] = lookup(s.slice(0, i).trim(), scope, root);
      const lit = s.slice(i + 2).trim();
      const eq = found && String(v) === lit;
      return op === "==" ? eq : !eq;
    }
  }
  if (s.startsWith("!")) {
    const [v, found] = lookup(s.slice(1).trim(), scope, root);
    return !found || !truthy(v);
  }
  const [v, found] = lookup(s, scope, root);
  return found && truthy(v);
}

export function evalWhen(expr, scope, root) {
  if (!expr || !expr.trim()) return true;
  return expr.split("||").some((and) => and.split("&&").every((term) => evalTerm(term, scope, root)));
}
