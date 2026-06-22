# Node + web example

The non-Go integration path: a Node backend using [`@calltoverify/sdk`](../../sdk-server-node)
against a **standalone Coordinator**, plus a tiny browser UI. The browser calls this Node server,
which calls the Coordinator with the SDK — so your API key never reaches the client.

(For Go backends, prefer the embedded engine — see [`coordinator/examples/dashboard`](../../coordinator/examples/dashboard).)

## Run

```bash
# 1. Start a Coordinator + Postgres + Redis (from the repo root):
docker compose up --build

# 2. Provision an app to get an API key (admin token is set in docker-compose.yml):
curl -s -X POST localhost:8080/admin/apps \
  -H 'Authorization: Bearer dev-admin-change-me' -d '{"name":"demo"}'
# -> copy the "api_key" from the response

# 3. Build the Node SDK this example imports:
( cd ../../sdk-server-node && npm install && npm run build )

# 4. Pair a receiver so verifications can complete:
#    provision a device + number via /admin, then run a receiver (receiver-pi or receiver-android)
#    pointed at this Coordinator. See those directories' READMEs.

# 5. Run the example:
CTV_API_KEY=<api_key-from-step-2> npm start
# open http://localhost:3000
```

## Configuration

| Env | Default | |
|---|---|---|
| `CTV_API_KEY` | _(required)_ | developer API key from `/admin/apps` |
| `COORDINATOR_URL` | `http://localhost:8080` | the Coordinator base URL |
| `PORT` | `3000` | this example's port |

## What it shows

- `server.mjs` proxies `POST /api/start` → `ctv.startVerification(...)` and
  `GET /api/status` → `ctv.checkStatus(...)`, keeping the API key server-side.
- `public/index.html` renders the instructions and polls until verified — the same flow the
  framework SDKs ([widget](../../widget-web), [React](../../sdk-client-react),
  [Flutter](../../sdk-client-flutter)) package up for you.
