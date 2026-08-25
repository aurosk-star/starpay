import { expect, test } from "bun:test";

const packageJSON = await Bun.file(
  new URL("../package.json", import.meta.url),
).json();

test("uses cmdk and does not depend on Clerk React", () => {
  expect(packageJSON.dependencies?.cmdk).toBeDefined();

  const dependencyNames = [
    ...Object.keys(packageJSON.dependencies ?? {}),
    ...Object.keys(packageJSON.devDependencies ?? {}),
    ...Object.keys(packageJSON.peerDependencies ?? {}),
    ...Object.keys(packageJSON.optionalDependencies ?? {}),
  ];
  expect(dependencyNames).not.toContain("@clerk/react");
});
