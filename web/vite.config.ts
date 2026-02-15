import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    proxy: {
      "/health": "http://localhost:8080",
      "/api": "http://localhost:8080"
    }
  }
});
