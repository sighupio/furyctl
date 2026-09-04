/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// The two helpers every module here builds its DOM with. Their own module so the renderer and the
// file editor can both use them without importing each other.

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
