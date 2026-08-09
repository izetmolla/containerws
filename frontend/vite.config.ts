import path from "path"
import fs from "fs"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig, type Plugin } from "vite"

// https://vite.dev/config/
export default defineConfig(async ({ command }) => {
  return {
    plugins: [react(), tailwindcss(), replaceTags(command)],
    resolve: {
      alias: {
        "@": path.resolve(import.meta.dirname, "./src"),
      },
    },
    base: process.env.NODE_ENV === "production" ? "/static/" : "/",
    build: {
      assetsDir: "assets", // Ensures assets are placed inside /static/assets/
      outDir: "static",
      manifest: true,
      rollupOptions: {
        output: {
          manualChunks(id: string) {
            if (id.includes("node_modules/lucide-react")) {
              return "lucide"
            }
          },
        },
      },
    },
    server: {
      port: 5171,
      host: "0.0.0.0",
      allowedHosts: [
        "localhost",
        "127.0.0.1",
        "0.0.0.0",
        "ppa-dev.izetmolla.com",
      ],
        proxy: {
          "/api": {
            target: "http://localhost:8999",
            changeOrigin: true,
            ws: true,
          },
          "/mcp": {
            target: "http://localhost:8999",
            changeOrigin: true,
            ws: true,
          },
          // Full noVNC proxy → Fiber → :6080 (HTTP + WebSocket)
          "/novnc": {
            target: "http://localhost:8999",
            changeOrigin: true,
            ws: true,
          },
          "/codeserver": {
            target: "http://localhost:8999",
            changeOrigin: true,
            ws: true,
          },
      },
    },
  }
})

function replaceTags(command: string): Plugin {
  return {
    name: "custom-html-transform",
    enforce: "pre",
    transformIndexHtml(html: string) {
      // Paths must be real files for Vite in both serve and build.
      let out = html
        .replace(/{{.theme_url}}\/src\/main\.tsx/g, "/src/main.tsx")
        .replace(/{{.base_url}}\/favicon\.svg/g, "/favicon.svg")

      // Dev-only: substitute Go template placeholders so the dev server renders.
      // On build, keep {{.title}}, {{.globalOptions}}, etc. for server-side template execution.
      if (command === "serve") {
        out = out
          .replace(/{{.title}}/g, "ContainerWS Admin Console")
          .replace(
            /{{.globalOptions}}/g,
            '<script id="__GLOBAL_DATA__" data-app="app" type="application/json"></script>'
          )
          .replace(
            /{{.globalContent}}/g,
            '<script id="__GLOBAL_CONTENT_DATA__" type="application/json"></script>'
          )
          .replace(/{{ .metaData }}/g, "")
          .replace(/{{if .metaData }}/g, "")
          .replace(/{{else}}/g, "")
          .replace(/{{end}}/g, "")
          .replace(/{{if .metaData }}/g, "")
          .replace(/{{else}}/g, "")
          .replace(/{{end}}/g, "")
          .replace(/{{if .metaData }}/g, "")
      }

      return out
    },
    closeBundle() {
      if (command !== "build") return

      const distIndex = path.resolve(import.meta.dirname, "static/index.html")
      if (!fs.existsSync(distIndex)) return

      let patched = fs.readFileSync(distIndex, "utf8")
      patched = patched.replace(/(["'])\/static\//g, "$1{{.theme_url}}/static/")
      patched = patched.replace(
        /href="\/favicon\.svg"/g,
        'href="{{.base_url}}/favicon.svg"'
      )
      fs.writeFileSync(distIndex, patched, "utf8")
    },
  }
}