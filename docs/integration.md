# Wire it into your app — front end + back end

This is the actual code that goes in your app. The browser never talks to CallToVerify directly (your API key must stay on your server), so the shape is always:

```
[ Browser — your UI ]  ⇄  [ Your backend — 2 routes ]  ⇄  [ CallToVerify ]
```

You add **two backend routes** and drop a **widget** (or ~20 lines) into your front end. That's the whole integration. Below is each side.

## Back end — two routes

Expose `POST /api/start` (trigger a verification) and `GET /api/status` (read the result). Pick the snippet for your stack.

### Node / TypeScript — standalone Coordinator

```js
import express from "express";
import { CallToVerify } from "@calltoverify/sdk";

const ctv = new CallToVerify({
  baseUrl: process.env.COORDINATOR_URL,   // your Coordinator
  apiKey: process.env.CTV_API_KEY,        // from POST /admin/apps — stays server-side
});
const app = express();

// Trigger → { sessionId, instructions: { number, code, action, deepLink, expiresAt } }
app.post("/api/start", async (req, res) =>
  res.json(await ctv.startVerification({ channel: req.query.channel || "sms" })));

// Read the result → { status: "pending" | "verified" | "expired", verifiedMsisdn }
app.get("/api/status", async (req, res) =>
  res.json(await ctv.checkStatus(req.query.id)));

app.listen(3000);
```

The Python (`calltoverify`) and PHP/Laravel SDKs expose the same `startVerification` / `checkStatus`.

### Go — embedded engine (no separate service, no API key)

```go
// once at startup
eng, _ := ctv.New(ctx, ctv.Options{
    SQLitePath: "calltoverify.db",                 // or PostgresDSN in production
    OnVerified: func(ev ctv.Event) {
        // ev.VerifiedMSISDN is verified — persist it (this is your source of truth)
    },
})
mux.Handle("/ctv/", eng.DeviceHandler("/ctv"))     // receivers post here

mux.HandleFunc("POST /api/start", func(w http.ResponseWriter, r *http.Request) {
    v, _ := eng.StartVerification(r.Context(), ctv.Params{Channel: r.URL.Query().Get("channel")})
    writeJSON(w, map[string]any{"sessionId": v.SessionID, "instructions": v.Instructions})
})
mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
    st, _ := eng.Status(r.Context(), r.URL.Query().Get("id"))
    writeJSON(w, map[string]any{"status": st.Status, "verifiedMsisdn": st.VerifiedMSISDN})
})
```

## Front end — the drop-in widget (easiest)

Point the widget's `start` / `status` at the two routes above. It renders the instructions, polls for you, and shows the success state.

```html
<div id="verify"></div>
<script type="module">
  import { mount } from "@calltoverify/widget";

  mount("#verify", {
    channels: ["sms"],                                       // or ["sms","call"] to offer a choice
    start:  (channel)   => fetch(`/api/start?channel=${channel}`, { method: "POST" }).then(r => r.json()),
    status: (sessionId) => fetch(`/api/status?id=${sessionId}`).then(r => r.json()),
    onVerified: (number) => {
      // `number` is the user's verified phone number. Advance your UI / fetch your session.
      console.log("verified:", number);
    },
  });
</script>
```

React and Flutter components take the same `start` / `status` / `onVerified`.

### …or hand-roll it (no dependency)

The same loop in ~20 lines of vanilla JS — useful to see exactly what the widget does:

```html
<div id="view"><button onclick="start()">Verify my number</button></div>
<script>
async function start() {
  const v = await fetch("/api/start?channel=sms", { method: "POST" }).then(r => r.json());
  const view = document.getElementById("view");
  const i = v.instructions;
  view.innerHTML =
    `<p>${i.action}</p>` +                                    // "Text 4729 to +8801700000001"
    `<div class="code">${[...(i.code || "")].map(d => `<span>${d}</span>`).join("")}</div>` +
    `<a href="${i.deepLink}">Open messages</a><p id="s">Waiting for your text…</p>`;

  const timer = setInterval(async () => {                     // poll until it flips
    const st = await fetch(`/api/status?id=${v.sessionId}`).then(r => r.json());
    if (st.status === "verified") {
      clearInterval(timer);
      view.innerHTML = `<h2>Verified ✓</h2><p>${st.verifiedMsisdn}</p>`;   // the user's number
    } else if (st.status === "expired") {
      clearInterval(timer);
      document.getElementById("s").textContent = "Expired. Try again.";
    }
  }, 2000);
}
</script>
```

A runnable version of exactly this is in [`examples/node-web`](../examples/node-web).

## Where does the user's number come from?

This trips people up, so to be explicit:

- **Default (`derive` mode):** you do **not** ask the user to type their number. They text or call you *from* the number they're proving, and that number comes back to you as **`verifiedMsisdn`** — in the widget's `onVerified(number)`, in `checkStatus(...).verifiedMsisdn`, and server-side in the `OnVerified` callback / webhook. The number is *proven*, not typed. Best for sign-up.
- **`claim` mode:** if you already have a number to confirm (the user typed it, or you have it on file), pass it when you start and the inbound must come from exactly that number:
  - Node: `ctv.startVerification({ channel: "sms", bindingMode: "claim", claimedMsisdn: "+8801712345678" })`
  - Go: `ctv.Params{Channel: "sms", BindingMode: "claim", ClaimedMSISDN: "+8801712345678"}`

  The result is the same `verified` / `expired`; the difference is whether the number was discovered or asserted up front.

## Recording the result (do this server-side)

The browser is convenient for *showing* the update, but don't trust it for *recording* it. Mark the user verified on your server, where it can't be forged:

- **Go (embedded):** the `OnVerified(ev)` callback fires with `ev.VerifiedMSISDN` — write it to your DB there.
- **Standalone:** set a `webhook_url` on the app; CallToVerify POSTs a signed `verification.verified` event. Verify the signature (the Node SDK's `verifyWebhook(rawBody, signature)` does this) before trusting it. Polling `checkStatus` from your status route also works for simpler setups.

## Next

- Set up the backend it talks to: [`getting-started.md`](getting-started.md).
- Channels (SMS / missed call / DTMF) and binding modes: [`channels.md`](channels.md).
- Runnable Node + browser example: [`examples/node-web`](../examples/node-web).
