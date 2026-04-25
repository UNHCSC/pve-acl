import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
    base: "/static/build/",
    plugins: [react()],
    build: {
        emptyOutDir: true,
        manifest: false,
        outDir: "static/build",
        rollupOptions: {
            input: "src/main.tsx",
            watch: {
                exclude: ["node_modules/**", "static/build/**"]
            },
            output: {
                assetFileNames: (assetInfo) => {
                    const name = assetInfo.name || assetInfo.names.find(Boolean) || assetInfo.originalFileName || assetInfo.originalFileNames.find(Boolean) || "";
                    if (name.endsWith(".css")) {
                        return "site.css";
                    }

                    return "[name][extname]";
                },
                entryFileNames: "site.js"
            }
        }
    }
});
