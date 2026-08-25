import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { resolveAPIBaseURL } from "../src/lib/api-base-url.ts";

test("uses same-origin API paths in production by default", () => {
  assert.equal(resolveAPIBaseURL(undefined), "");
  assert.equal(resolveAPIBaseURL(""), "");
});

test("preserves the development proxy prefix", () => {
  assert.equal(resolveAPIBaseURL("/api"), "/api");
});

test("development proxy forwards the API paths used by the client", async () => {
  const config = await readFile(
    new URL("../rsbuild.config.ts", import.meta.url),
    "utf8",
  );
  assert.match(config, /["']\/v1["']\s*:/);
});
