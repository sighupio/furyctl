/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import { describe, expect, test } from "bun:test";
import { hostnameFor, incrementIp, merge, reconcile, resolve, roles } from "../nodes.js";

const topology = {
  cpCount: 3,
  dedicatedEtcd: false,
  lbMode: "dedicated",
  lbCount: 2,
  infraCount: 3,
  workersCount: 2,
  extraGroups: [{ name: "gpu", count: 1, labels: [], taints: [] }],
};

test("roles follow the topology", () => {
  expect(roles(topology)).toEqual([
    { role: "lb", group: "", count: 2 },
    { role: "cp", group: "", count: 3 },
    { role: "infra", group: "infra", count: 3 },
    { role: "workers", group: "workers", count: 2 },
    { role: "custom", group: "gpu", count: 1 },
  ]);
  expect(roles({ ...topology, dedicatedEtcd: true, lbMode: "none" }).map((r) => r.role)).toEqual([
    "cp",
    "etcd",
    "infra",
    "workers",
    "custom",
  ]);
  expect(roles({ ...topology, lbMode: "keepalived-on-cp", infraCount: 0 }).map((r) => r.role)).toEqual([
    "cp",
    "workers",
    "custom",
  ]);
});

test("hostnameFor", () => {
  expect(hostnameFor("cp", "", 1, "k8s.example.com")).toBe("cp1.k8s.example.com");
  expect(hostnameFor("custom", "gpu", 2, "k8s.example.com")).toBe("gpu2.k8s.example.com");
});

test("incrementIp", () => {
  expect(incrementIp("192.168.1.10", 2)).toBe("192.168.1.12");
  expect(incrementIp("192.168.1.255", 1)).toBe("192.168.2.0");
  expect(incrementIp("", 1)).toBe("");
  expect(incrementIp("garbage", 1)).toBe("");
});

describe("reconcile", () => {
  test("creates rows with prefilled hostnames", () => {
    const state = reconcile({ rows: [], roleOverrides: {}, firstIp: {} }, topology, "k8s.example.com");
    expect(state.rows).toHaveLength(11);
    expect(state.rows[0]).toMatchObject({ role: "lb", index: 1, hostname: "lb1.k8s.example.com", macAddress: "", ip: "" });
    expect(state.rows[10]).toMatchObject({ role: "custom", group: "gpu", hostname: "gpu1.k8s.example.com" });
  });

  test("keeps edited rows and trims removed ones", () => {
    let state = reconcile({ rows: [], roleOverrides: {}, firstIp: {} }, topology, "k8s.example.com");
    state.rows[2].macAddress = "52:54:00:01:00:01";
    state = reconcile(state, { ...topology, cpCount: 1, lbMode: "none" }, "k8s.example.com");
    expect(state.rows.filter((r) => r.role === "cp")).toHaveLength(1);
    expect(state.rows.find((r) => r.role === "cp").macAddress).toBe("52:54:00:01:00:01");
    expect(state.rows.some((r) => r.role === "lb")).toBe(false);
  });

  test("fills ips from firstIp when a row has none", () => {
    const state = reconcile({ rows: [], roleOverrides: {}, firstIp: { cp: "192.168.1.10" } }, topology, "k8s.example.com");
    expect(state.rows.filter((r) => r.role === "cp").map((r) => r.ip)).toEqual([
      "192.168.1.10",
      "192.168.1.11",
      "192.168.1.12",
    ]);
  });
});

test("merge replaces arrays and deep-merges objects", () => {
  expect(merge({ a: 1, list: [1, 2], adv: { x: 1, y: 2 } }, { list: [3], adv: { y: 3 } }, { a: 2 })).toEqual({
    a: 2,
    list: [3],
    adv: { x: 1, y: 3 },
  });
});

test("resolve applies defaults, role override, node override", () => {
  const defaults = { arch: "x86-64", installDisk: "/dev/sda", advanced: { disks: [] } };
  const state = {
    rows: [
      { role: "lb", group: "", index: 1, hostname: "lb1.x", macAddress: "aa", ip: "10.0.0.1", overrides: {} },
      {
        role: "cp",
        group: "",
        index: 1,
        hostname: "cp1.x",
        macAddress: "bb",
        ip: "10.0.0.2",
        overrides: { installDisk: "/dev/nvme0n1" },
      },
    ],
    roleOverrides: { lb: { enabled: true, values: { arch: "arm64" } }, cp: { enabled: false, values: { arch: "arm64" } } },
    firstIp: {},
  };
  const out = resolve(state, defaults);
  expect(out[0]).toMatchObject({
    hostname: "lb1.x",
    macAddress: "aa",
    ip: "10.0.0.1",
    role: "lb",
    group: "",
    arch: "arm64",
    installDisk: "/dev/sda",
  });
  expect(out[1]).toMatchObject({ hostname: "cp1.x", arch: "x86-64", installDisk: "/dev/nvme0n1" });
  expect(out[1].advanced).toEqual({ disks: [] });
});
