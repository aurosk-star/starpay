import path from "node:path";
import { fileURLToPath } from "node:url";

import { defineConfig } from "@rsbuild/core";
import { pluginReact } from "@rsbuild/plugin-react";

const dirname = path.dirname(fileURLToPath(import.meta.url));

// Docs: https://rsbuild.rs/config/
export default defineConfig(() => {
  const serverUrl = process.env.VITE_BACKEND_URL || "http://localhost:8080";

  return {
    plugins: [pluginReact()],
    source: {
      entry: {
        index: "./src/index.tsx",
      },
    },
    html: {
      title: "starpay-支付网关",
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
          pathRewrite: {
            "^/api": "",
          },
        },
      },
    },
  };
});
