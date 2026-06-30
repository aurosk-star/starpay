import path from "node:path";
import { fileURLToPath } from "node:url";

import { defineConfig, loadEnv } from "@rsbuild/core";
import { pluginReact } from "@rsbuild/plugin-react";

const dirname = path.dirname(fileURLToPath(import.meta.url));

// Docs: https://rsbuild.rs/config/
export default defineConfig(({ envMode }) => {
  const env = loadEnv({ mode: envMode, prefixes: ["VITE_"] });
  const serverUrl =
    process.env.VITE_API_BASE_URL ||
    env.rawPublicVars.VITE_API_BASE_URL ||
    "http://localhost:8080";

  return {
    plugins: [pluginReact()],
    source: {
      entry: {
        index: "./src/index.tsx",
      },
    },
    resolve: {
      alias: {
        "@": path.resolve(dirname, "./src"),
      },
    },
    server: {
      host: "0.0.0.0",
      proxy: {
        "/api": {
          target: serverUrl,
          changeOrigin: true,
        },
      },
    },
  };
});
