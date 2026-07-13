export function buildReconciliationSearch(
  params: Record<string, string | number | undefined>,
) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "" || value === "all") continue;
    search.set(key, String(value));
  }
  return search.toString();
}
