/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import { expect, test } from "bun:test";
import { entriesFor, labelFor, mountFor } from "../datadisk.js";

test("a purpose brings its documented mount point and label", () => {
  expect(mountFor("containerd")).toBe("/var/lib/containerd");
  expect(mountFor("kubelet")).toBe("/var/lib/kubelet");
  expect(mountFor("etcd")).toBe("/var/lib/etcd");
  expect(labelFor("containerd")).toBe("containerd");
});

test("a custom mount point derives its label from the last path segment", () => {
  expect(mountFor("custom", "/var/lib/mydata")).toBe("/var/lib/mydata");
  expect(labelFor("custom", "/var/lib/mydata")).toBe("mydata");
  expect(labelFor("custom", "/srv/data files")).toBe("data-files");
  expect(labelFor("custom", "")).toBe("data");
  // A partition label cannot contain a colon and stops at 36 characters.
  expect(labelFor("custom", `/srv/${"x".repeat(60)}`)).toHaveLength(36);
  expect(labelFor("custom", "/srv/a:b")).not.toContain(":");
});

test("the disk and the filesystem reference each other by partition label", () => {
  const { disk, filesystem } = entriesFor({
    device: "/dev/disk/by-id/scsi-0QEMU_drive-scsi1",
    purpose: "containerd",
    mount: "",
    format: "ext4",
  });

  expect(disk).toEqual({
    device: "/dev/disk/by-id/scsi-0QEMU_drive-scsi1",
    wipeTable: true,
    partitions: [{ label: "containerd", number: 1, sizeMib: "", startMib: "", typeGuid: "", shouldExist: true, resize: false }],
  });

  expect(filesystem).toEqual({
    device: "/dev/disk/by-partlabel/containerd",
    format: "ext4",
    path: "/var/lib/containerd",
    label: "containerd",
    wipeFilesystem: false,
    withMountUnit: true,
    options: [],
    mountOptions: [],
  });
});
