/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import { expect, test } from "bun:test";
import { collectFields } from "../renderer.js";

const fields = [
  { id: "name", type: "text" },
  { id: "count", type: "number" },
  { id: "labels", type: "keyValue" },
  { id: "args", type: "list", item: { type: "text" } },
  { id: "types", type: "list", item: { type: "choice", options: ["audit", "api"] } },
  { id: "solvers", type: "yaml" },
  { id: "hidden", type: "text", when: "name == nobody" },
];

test("a keyValue is edited as pairs and collected as a map", () => {
  const out = collectFields(
    fields,
    {
      name: "production",
      count: "3",
      labels: [
        { key: "accelerator", value: "nvidia" },
        { key: "", value: "orphan" },
      ],
      args: ["console=ttyS0", ""],
      types: ["audit"],
      solvers: "- dns01: {}",
      hidden: "never",
    },
    {},
  );

  expect(out.labels).toEqual({ accelerator: "nvidia" });
  expect(out.args).toEqual(["console=ttyS0"]);
  expect(out.types).toEqual(["audit"]);
  expect(out.count).toBe(3);
  expect(out.solvers).toBe("- dns01: {}");
  expect(out).not.toHaveProperty("hidden");
});

test("an untouched keyValue is an empty map, not a missing key", () => {
  const out = collectFields([{ id: "labels", type: "keyValue" }], {}, {});
  expect(out.labels).toEqual({});
});
