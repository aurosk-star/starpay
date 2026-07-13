import assert from "node:assert/strict";
import test from "node:test";

import { resolveAPIBaseURL } from "../src/lib/api-base-url.ts";

test("uses same-origin API paths in production by default", () => {
  assert.equal(resolveAPIBaseURL(undefined), "");
  assert.equal(resolveAPIBaseURL(""), "");
});

test("preserves the development proxy prefix", () => {
  assert.equal(resolveAPIBaseURL("/api"), "/api");
});
