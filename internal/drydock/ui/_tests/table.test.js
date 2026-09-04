/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import { expect, test } from "bun:test";
import { collectFields, initScope, missingRequired } from "../renderer.js";

// The shape an EKS node pool has in the schema, written the way a wizard would: flat fields that
// the template nests back into size{} and instance{}. The columns are only how it looks.
const pools = {
  id: "nodePools",
  type: "table",
  config: { columns: "name,instanceType,min,max" },
  fields: [
    { id: "name", type: "text", required: true },
    { id: "type", type: "choice", options: ["eks-managed", "self-managed"], default: "eks-managed" },
    { id: "instanceType", type: "text", required: true },
    { id: "min", type: "number", default: 1 },
    { id: "max", type: "number", default: 3 },
    { id: "spot", type: "bool" },
    { id: "labels", type: "keyValue" },
  ],
};

test("a table holds the same answers as a list of groups", () => {
  const scope = initScope([pools], {});
  expect(scope.nodePools).toEqual([]);

  scope.nodePools.push(initScope(pools.fields, {}));
  Object.assign(scope.nodePools[0], { name: "infra", instanceType: "m5.xlarge", min: 3, max: 3 });
  scope.nodePools[0].labels.push({ key: "node-role.kubernetes.io/infra", value: "" });

  const out = collectFields([pools], scope, {});
  expect(out.nodePools).toEqual([
    {
      name: "infra",
      type: "eks-managed",
      instanceType: "m5.xlarge",
      min: 3,
      max: 3,
      spot: false,
      labels: { "node-role.kubernetes.io/infra": "" },
    },
  ]);
});

test("a required cell that is empty is still reported, row by row", () => {
  const scope = initScope([pools], {});
  scope.nodePools.push(initScope(pools.fields, {}), initScope(pools.fields, {}));
  scope.nodePools[0].name = "infra";
  scope.nodePools[0].instanceType = "m5.xlarge";
  scope.nodePools[1].name = "workers";

  expect(missingRequired([pools], scope, {})).toEqual(["nodePools 2: instanceType"]);
});
