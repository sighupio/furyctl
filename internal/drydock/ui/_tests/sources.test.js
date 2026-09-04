/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import { describe, expect, test } from "bun:test";
import { collectReferences, decode, encode, suggestName } from "../sources.js";

describe("decode", () => {
  test("plain value", () => expect(decode("abc")).toEqual({ source: "value", raw: "abc" }));
  test("env", () => expect(decode("{env://FOO}")).toEqual({ source: "env", raw: "FOO" }));
  test("file", () => expect(decode("{file://./a.txt}")).toEqual({ source: "file", raw: "./a.txt" }));
  test("path", () => expect(decode("{path://./pki}")).toEqual({ source: "path", raw: "./pki" }));
  test("https keeps the scheme in raw", () =>
    expect(decode("{https://x.io/a}")).toEqual({ source: "http", raw: "https://x.io/a" }));
  test("non-string", () => expect(decode(3)).toEqual({ source: "value", raw: 3 }));
  // Picking the URL source stores `{}`: without the scheme there is nothing else to recognise it by,
  // so an address still being typed has to keep the source rather than turn into a literal `{…}`.
  test("a url without its scheme yet is still a url", () => {
    expect(decode("{}")).toEqual({ source: "http", raw: "" });
    expect(decode("{example.io/ca.crt}")).toEqual({ source: "http", raw: "example.io/ca.crt" });
  });
});

describe("encode", () => {
  test("round trips every source", () => {
    for (const v of ["abc", "{env://FOO}", "{file://./a}", "{path://./p}", "{http://x}", "{https://x}"]) {
      const d = decode(v);
      expect(encode(d.source, d.raw)).toBe(v);
    }
  });
});

test("suggestName is SCREAMING_SNAKE with the cluster prefix", () => {
  expect(suggestName("sshPrivateKey", "prod-eu")).toBe("PROD_EU_SSH_PRIVATE_KEY");
  expect(suggestName("lokiSecretAccessKey", "")).toBe("LOKI_SECRET_ACCESS_KEY");
});

test("collectReferences walks nested answers and dedupes", () => {
  const refs = collectReferences({
    cluster: { sshPrivateKey: "{env://KEY}", proxy: { http: "" } },
    nodes: [{ macAddress: "{env://CP1_MAC}" }, { macAddress: "{env://CP1_MAC}" }],
    kubernetes: { encryptionConfig: "{file://./secrets/enc.yaml}" },
  });
  expect(refs).toEqual([
    { source: "env", raw: "KEY", path: "cluster.sshPrivateKey" },
    { source: "env", raw: "CP1_MAC", path: "nodes.0.macAddress" },
    { source: "file", raw: "./secrets/enc.yaml", path: "kubernetes.encryptionConfig" },
  ]);
});
