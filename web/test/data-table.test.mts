import assert from "node:assert/strict";
import test from "node:test";

test("data table keeps the typed factory and optional slots", async () => {
  const source = await Bun.file(
    new URL("../src/components/data-table/factory.tsx", import.meta.url),
  ).text();
  assert.match(source, /createDataTable<TData>/);
  assert.match(source, /toolbar\?:/);
  assert.match(source, /pagination\?:/);
  assert.match(source, /viewOptions\?:/);
  assert.match(source, /emptyState\?:/);
});

test("pagination utility keeps compact ranges and ellipses", async () => {
  const source = await Bun.file(
    new URL("../src/components/data-table/pagination.tsx", import.meta.url),
  ).text();
  assert.match(source, /export function getPaginationItems/);
  assert.match(source, /"ellipsis"/);
});

test("data table exports reusable shadcn controls", async () => {
  const source = await Bun.file(
    new URL("../src/components/data-table/index.ts", import.meta.url),
  ).text();
  for (const name of [
    "DataTableToolbar",
    "DataTableViewOptions",
    "DataTableColumnHeader",
    "DataTablePagination",
  ]) {
    assert.match(source, new RegExp(name));
  }
});
