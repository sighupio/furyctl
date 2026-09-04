/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// The editor behind a `{file://…}` value: writes the file the configuration refers to, so the
// operator does not have to leave the wizard to prepare an encryption manifest or a certificate.
//
// The server decides what may be written (under the configuration's own directory, mode 0600, never
// replacing a file unless asked); this only asks for the path and the content.

import { t } from "./i18n.js";
import { button, el } from "./dom.js";

/** What was written in this session, so the review can say so. Keyed by the path in the file. */
export const created = new Map();

export function openFileEditor({ path = "", content = "", onSaved } = {}) {
  const backdrop = el("div", "modal-backdrop");
  const modal = el("div", "modal");
  modal.setAttribute("role", "dialog");
  modal.setAttribute("aria-modal", "true");

  const head = el("div", "modal-head");
  head.append(Object.assign(el("h3"), { textContent: t("file.title") }));
  modal.append(head);

  const pathField = el("div", "wz-field");
  pathField.append(Object.assign(el("label"), { textContent: t("file.path") }));
  const pathInput = el("input", "wz-input");
  pathInput.type = "text";
  pathInput.value = path;
  pathInput.placeholder = "./secrets/etcd-encryption-config.yaml";
  pathField.append(pathInput, Object.assign(el("span", "help"), { textContent: t("file.hint") }));
  modal.append(pathField);

  const contentField = el("div", "wz-field");
  contentField.append(Object.assign(el("label"), { textContent: t("file.content") }));
  const area = el("textarea", "wz-input wz-yaml modal-editor");
  area.value = content;
  area.spellcheck = false;
  contentField.append(area);
  modal.append(contentField);

  const error = el("p", "error");
  error.hidden = true;
  modal.append(error);

  const close = () => {
    backdrop.remove();
    document.removeEventListener("keydown", onKey);
  };

  function onKey(e) {
    if (e.key === "Escape") close();
  }

  const save = async (overwrite) => {
    error.hidden = true;
    saveButton.disabled = true;
    try {
      const res = await fetch("/api/file", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path: pathInput.value.trim(), content: area.value, overwrite }),
      });
      const data = await res.json().catch(() => ({}));

      if (res.status === 409) {
        // The file is already there: replacing it is a second, explicit click.
        error.hidden = false;
        error.textContent = t("file.exists");
        replaceButton.hidden = false;
        saveButton.disabled = false;

        return;
      }

      if (!res.ok) throw new Error(data.error ?? `${res.status}`);

      created.set(pathInput.value.trim(), data.path);
      onSaved?.(pathInput.value.trim(), data.path);
      close();
    } catch (e) {
      error.hidden = false;
      error.textContent = e.message;
      saveButton.disabled = false;
    }
  };

  const foot = el("div", "modal-foot");
  const saveButton = button(t("file.save"), "btn primary", () => save(false));
  const replaceButton = button(t("file.replace"), "btn ghost", () => save(true));
  replaceButton.hidden = true;
  foot.append(button(t("file.cancel"), "btn ghost", close), el("span", "wz-grow"), replaceButton, saveButton);
  modal.append(foot);

  backdrop.append(modal);
  backdrop.addEventListener("click", (e) => {
    if (e.target === backdrop) close();
  });
  document.addEventListener("keydown", onKey);
  document.body.append(backdrop);
  area.focus();
}
