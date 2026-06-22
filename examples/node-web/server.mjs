// Minimal Node backend showing the non-Go integration path: the @calltoverify/sdk
// talks to a standalone Coordinator, and this server proxies start/status for the
// browser so the API key stays server-side. It also serves a tiny verification UI.
//
// See README.md for the full run instructions.
import http from "node:http";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { CallToVerify } from "../../sdk-server-node/dist/index.js";

const dir = dirname(fileURLToPath(import.meta.url));

const ctv = new CallToVerify({
  baseUrl: process.env.COORDINATOR_URL ?? "http://localhost:8080",
  apiKey: process.env.CTV_API_KEY ?? "",
});

const send = (res, code, obj) => {
  res.writeHead(code, { "content-type": "application/json" });
  res.end(JSON.stringify(obj));
};

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, "http://localhost");
  try {
    if (req.method === "POST" && url.pathname === "/api/start") {
      return send(res, 200, await ctv.startVerification({ channel: url.searchParams.get("channel") ?? "sms" }));
    }
    if (req.method === "GET" && url.pathname === "/api/status") {
      return send(res, 200, await ctv.checkStatus(url.searchParams.get("id") ?? ""));
    }
    if (req.method === "GET" && (url.pathname === "/" || url.pathname === "/index.html")) {
      res.writeHead(200, { "content-type": "text/html; charset=utf-8" });
      return res.end(await readFile(join(dir, "public", "index.html")));
    }
    res.writeHead(404);
    res.end("not found");
  } catch (err) {
    send(res, err.status ?? 500, { error: String(err.message ?? err) });
  }
});

const port = process.env.PORT ?? 3000;
server.listen(port, () => console.log(`node-web example on http://localhost:${port}`));
