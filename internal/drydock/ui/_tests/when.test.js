import { expect, test } from "bun:test";
import { evalWhen } from "../when.js";

const root = {
  topology: { lbMode: "dedicated", lbCount: 2, dedicatedEtcd: false },
  modules: { networking: { type: "cilium" } },
};
const scope = root.topology;

// Keep identical to TestWhenEval in when_test.go.
const cases = [
  ["lbMode == dedicated", true],
  ["lbMode != dedicated", false],
  ["lbCount == 2", true],
  ["lbCount == 1", false],
  ["dedicatedEtcd", false],
  ["!dedicatedEtcd", true],
  ["lbMode == dedicated && lbCount == 2", true],
  ["lbMode == dedicated && lbCount == 1", false],
  ["lbMode == none || lbCount == 2", true],
  ["lbMode == none || lbCount == 1", false],
  ["lbMode == none || lbMode == dedicated && lbCount == 2", true],
  ["topology.lbMode == dedicated", true],
  ["modules.networking.type == cilium", true],
  ["missing", false],
  ["!missing", true],
  ["missing == x", false],
  ["missing != x", true],
  ["", true],
];

for (const [expr, want] of cases) {
  test(`when: ${expr || "(empty)"}`, () => expect(evalWhen(expr, scope, root)).toBe(want));
}
