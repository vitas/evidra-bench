import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  base: process.env.VITE_BASE_PATH || "/",
  plugins: [react(), tailwindcss()],
  server: {
    proxy: process.env.VITE_DEV_API_PROXY
      ? {
          "/v1": {
            target: process.env.VITE_DEV_API_PROXY,
            changeOrigin: true,
            secure: true,
          },
        }
      : undefined,
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
