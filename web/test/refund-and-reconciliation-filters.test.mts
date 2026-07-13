import assert from "node:assert/strict";
import test from "node:test";

import { buildRefundSearch } from "../src/features/refunds/filters.ts";
import { buildReconciliationSearch } from "../src/features/reconciliations/filters.ts";

test("refund search omits all values and keeps concrete filters", () => {
  assert.equal(buildRefundSearch({ status: "all", channel: "all" }), "");
  assert.equal(
    buildRefundSearch({ status: "failed", gateway_order_no: "gw_1" }),
    "status=failed&gateway_order_no=gw_1",
  );
});

test("reconciliation search keeps manual recovery filters", () => {
  assert.equal(
    buildReconciliationSearch({ status: "manual_required", channel: "paypal" }),
    "status=manual_required&channel=paypal",
  );
});
