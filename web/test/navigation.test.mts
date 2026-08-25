import { expect, test } from "bun:test";

import { navigationUrls } from "../src/config/navigation";

test("navigation covers every administrative entry point", () => {
  expect(navigationUrls).toEqual(
    expect.arrayContaining([
      "/",
      "/apps",
      "/orders",
      "/webhooks",
      "/refunds",
      "/reconciliations",
      "/channels",
      "/routing",
      "/config/gateway",
      "/users",
    ]),
  );
  expect(navigationUrls.some((url) => url.startsWith("/checkout"))).toBe(false);
});
