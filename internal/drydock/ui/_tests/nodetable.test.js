/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import { expect, test } from "bun:test";
import { columnsOf, domainOf, makeNodeTable } from "../nodetable.js";

const steps = [
  { id: "cluster", fields: [{ id: "domain", type: "text" }] },
  { id: "topology", fields: [] },
  { id: "nodeDefaults", fields: [{ id: "installDisk", type: "text", required: true }] },
];

const root = {
  cluster: { domain: "k8s.example.com" },
  topology: { cpCount: 3, lbMode: "none", infraCount: 0, workersCount: 0, extraGroups: [] },
  nodeDefaults: { installDisk: "/dev/sda", networkMode: "static" },
};

const fieldWith = (columns, domainFrom = "cluster.domain") => ({
  id: "nodes",
  type: "nodeTable",
  config: {
    topologyStep: "topology",
    defaultsStep: "nodeDefaults",
    ...(domainFrom ? { domainFrom } : {}),
    ...(columns ? { columns } : {}),
  },
});

test("the default columns are the Immutable set", () => {
  expect(columnsOf(fieldWith())).toEqual(["hostname", "macAddress", "ip"]);
  expect(columnsOf(fieldWith("hostname, ip"))).toEqual(["hostname", "ip"]);
  // A column nobody knows is ignored rather than rendered as an empty cell.
  expect(columnsOf(fieldWith("hostname,nonsense"))).toEqual(["hostname"]);
});

test("a provider without PXE is not asked for MAC addresses", () => {
  const widget = makeNodeTable(steps);

  const withMac = widget.missing(fieldWith(), {}, root);
  expect(withMac.filter((m) => m.includes("MAC"))).toHaveLength(3);
  expect(withMac.filter((m) => m.includes("IP"))).toHaveLength(3);

  const withoutMac = widget.missing(fieldWith("hostname,ip"), {}, root);
  expect(withoutMac.filter((m) => m.includes("MAC"))).toHaveLength(0);
  expect(withoutMac.filter((m) => m.includes("IP"))).toHaveLength(3);
});

test("an address that comes from DHCP is not something left to fill", () => {
  const widget = makeNodeTable(steps);
  const dhcp = { ...root, nodeDefaults: { ...root.nodeDefaults, networkMode: "dhcp" } };
  expect(widget.missing(fieldWith("hostname,ip"), {}, dhcp)).toHaveLength(0);
});

test("without domainFrom the generated names stay short, as OnPremises wants them", () => {
  const widget = makeNodeTable(steps);

  const fqdn = {};
  widget.missing(fieldWith("hostname,ip"), fqdn, root);
  expect(fqdn.rows.map((r) => r.hostname)).toEqual(["cp1.k8s.example.com", "cp2.k8s.example.com", "cp3.k8s.example.com"]);

  const short = {};
  widget.missing(fieldWith("hostname,ip", null), short, root);
  expect(short.rows.map((r) => r.hostname)).toEqual(["cp1", "cp2", "cp3"]);
});

test("domainOf reads the path out of the answers, and tolerates it being absent", () => {
  expect(domainOf(fieldWith(), root)).toBe("k8s.example.com");
  expect(domainOf(fieldWith(null, null), root)).toBe("");
  expect(domainOf(fieldWith(null, "nope.nothing"), root)).toBe("");
});
