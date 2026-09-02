/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// "Add a data disk": four answers instead of the three coordinated Butane lists nobody
// remembers. It writes one disk with a single partition and the filesystem that mounts it,
// referenced by partition label because the filesystem UUID does not exist until first boot.
//
// The mount points offered are the ones "Installing on bare-metal nodes" recommends giving
// a dedicated volume: the container runtime, the kubelet, and etcd on control plane nodes.

import { t, text } from "./i18n.js";
import { button, el } from "./renderer.js";

const PURPOSES = {
  containerd: { path: "/var/lib/containerd", label: "containerd" },
  kubelet: { path: "/var/lib/kubelet", label: "kubelet" },
  etcd: { path: "/var/lib/etcd", label: "etcd" },
};

// A partition label cannot contain a colon and stops at 36 characters.
export function labelFor(purpose, mount) {
  if (PURPOSES[purpose]) return PURPOSES[purpose].label;
  const fromPath = (mount || "").split("/").filter(Boolean).pop() ?? "data";
  return fromPath.replace(/[^A-Za-z0-9_-]/g, "-").slice(0, 36) || "data";
}

export function mountFor(purpose, mount) {
  return PURPOSES[purpose]?.path ?? mount;
}

// The two entries a data disk turns into, ready to append to the raw lists.
export function entriesFor({ device, purpose, mount, format }) {
  const label = labelFor(purpose, mount);
  return {
    disk: {
      device,
      wipeTable: true,
      partitions: [
        { label, number: 1, sizeMib: "", startMib: "", typeGuid: "", shouldExist: true, resize: false },
      ],
    },
    filesystem: {
      device: `/dev/disk/by-partlabel/${label}`,
      format,
      path: mountFor(purpose, mount),
      label,
      wipeFilesystem: false,
      withMountUnit: true,
      options: [],
      mountOptions: [],
    },
  };
}

export function makeDataDisk() {
  return {
    render(container, field, state, root, onChange) {
      state.device ??= "";
      state.purpose ??= "containerd";
      state.format ??= "ext4";
      state.mount ??= "";

      const box = el("div", "datadisk");
      const rerender = () => this.render(container, field, state, root, onChange);

      const label = el("label");
      label.textContent = text(field.label) || t("disk.title");
      box.append(label);
      if (text(field.help)) box.append(Object.assign(el("p", "help"), { textContent: text(field.help) }));

      const device = el("input", "wz-input");
      device.type = "text";
      device.value = state.device;
      device.placeholder = "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_drive-scsi1";
      device.addEventListener("input", () => {
        state.device = device.value;
      });
      box.append(row(t("disk.device"), device, t("disk.deviceHelp")));

      const purpose = el("select", "wz-input wz-select");
      for (const key of ["containerd", "kubelet", "etcd", "custom"]) {
        const o = el("option");
        o.value = key;
        o.textContent = t(`disk.purpose.${key}`);
        o.selected = key === state.purpose;
        purpose.append(o);
      }
      purpose.addEventListener("change", () => {
        state.purpose = purpose.value;
        rerender();
      });
      box.append(row(t("disk.purpose"), purpose));

      if (state.purpose === "custom") {
        const mount = el("input", "wz-input");
        mount.type = "text";
        mount.value = state.mount;
        mount.placeholder = "/var/lib/mydata";
        mount.addEventListener("input", () => {
          state.mount = mount.value;
        });
        box.append(row(t("disk.mount"), mount));
      }

      const format = el("select", "wz-input wz-select");
      for (const key of ["ext4", "xfs", "btrfs"]) {
        const o = el("option");
        o.value = key;
        o.textContent = key;
        o.selected = key === state.format;
        format.append(o);
      }
      format.addEventListener("change", () => {
        state.format = format.value;
      });
      box.append(row(t("disk.format"), format));

      const add = button(t("disk.add"), "btn ghost", () => {
        if (!state.device || (state.purpose === "custom" && !state.mount)) return;
        const { disk, filesystem } = entriesFor(state);
        const scope = root[field.config.scopeStep];
        const target = field.config.group ? scope[field.config.group] : scope;
        (target[field.config.disks] ??= []).push(disk);
        (target[field.config.filesystems] ??= []).push(filesystem);
        state.device = "";
        state.mount = "";
        onChange();
        rerender();
      });
      const foot = el("div", "datadisk-foot");
      foot.append(add, Object.assign(el("span", "help"), { textContent: t("disk.result") }));
      box.append(foot);

      container.replaceChildren(box);
    },

    // A helper, not an answer: it has nothing of its own to write or to ask for.
    collect() {
      return undefined;
    },

    missing() {
      return [];
    },
  };
}

function row(label, control, help) {
  const wrap = el("div", "wz-field");
  const l = el("label");
  l.textContent = label;
  wrap.append(l, control);
  if (help) wrap.append(Object.assign(el("span", "help"), { textContent: help }));
  return wrap;
}
