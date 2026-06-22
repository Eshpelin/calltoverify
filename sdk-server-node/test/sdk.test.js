import test from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import { createHmac } from "node:crypto";
import { CallToVerify, CallToVerifyError } from "../dist/index.js";

function startServer(handler) {
  return new Promise((resolve) => {
    const server = http.createServer(handler);
    server.listen(0, () => resolve(server));
  });
}

const urlOf = (s) => `http://127.0.0.1:${s.address().port}`;

test("startVerification sends auth and maps the response", async () => {
  let seenAuth, seenBody, seenMethod, seenUrl;
  const server = await startServer((req, res) => {
    seenAuth = req.headers.authorization;
    seenMethod = req.method;
    seenUrl = req.url;
    let b = "";
    req.on("data", (c) => (b += c));
    req.on("end", () => {
      seenBody = b;
      res.writeHead(201, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          session_id: "sess1",
          status: "pending",
          instructions: {
            number: "+8801700000001",
            code: "123456",
            channel: "sms",
            action: "Send 123456 to +8801700000001",
            deep_link: "sms:+8801700000001?body=123456",
            expires_at: "2026-01-01T00:00:00Z",
          },
        }),
      );
    });
  });

  const ctv = new CallToVerify({ baseUrl: urlOf(server), apiKey: "k" });
  const v = await ctv.startVerification({ channel: "sms", bindingMode: "derive" });

  assert.equal(seenAuth, "Bearer k");
  assert.equal(seenMethod, "POST");
  assert.equal(seenUrl, "/v1/verifications");
  assert.equal(JSON.parse(seenBody).channel, "sms");
  assert.equal(JSON.parse(seenBody).binding_mode, "derive");
  assert.equal(v.sessionId, "sess1");
  assert.equal(v.instructions.deepLink, "sms:+8801700000001?body=123456");
  assert.equal(v.instructions.code, "123456");
  server.close();
});

test("checkStatus maps a verified session", async () => {
  const server = await startServer((req, res) => {
    assert.equal(req.url, "/v1/verifications/sess1");
    res.writeHead(200, { "content-type": "application/json" });
    res.end(
      JSON.stringify({
        session_id: "sess1",
        status: "verified",
        channel: "sms",
        verified_msisdn: "+8801712345678",
        expires_at: "2026-01-01T00:00:00Z",
      }),
    );
  });

  const ctv = new CallToVerify({ baseUrl: urlOf(server), apiKey: "k" });
  const s = await ctv.checkStatus("sess1");
  assert.equal(s.status, "verified");
  assert.equal(s.verifiedMsisdn, "+8801712345678");
  server.close();
});

test("non-2xx maps to CallToVerifyError", async () => {
  const server = await startServer((req, res) => {
    res.writeHead(401, { "content-type": "application/json" });
    res.end(JSON.stringify({ error: "unauthorized", detail: "invalid API key" }));
  });

  const ctv = new CallToVerify({ baseUrl: urlOf(server), apiKey: "bad" });
  await assert.rejects(
    () => ctv.startVerification(),
    (err) => {
      assert.ok(err instanceof CallToVerifyError);
      assert.equal(err.status, 401);
      assert.equal(err.code, "unauthorized");
      return true;
    },
  );
  server.close();
});

test("verifyWebhook accepts a valid signature and rejects a bad one", () => {
  const secret = "whsec_test";
  const body = JSON.stringify({
    event: "verification.verified",
    session_id: "sess1",
    verified_msisdn: "+8801712345678",
    channel: "sms",
    ts: "2026-01-01T00:00:00Z",
  });
  const sig = createHmac("sha256", secret).update(body).digest("hex");

  const ctv = new CallToVerify({ baseUrl: "http://unused", apiKey: "k", webhookSecret: secret });
  const ev = ctv.verifyWebhook(body, sig);
  assert.equal(ev.sessionId, "sess1");
  assert.equal(ev.verifiedMsisdn, "+8801712345678");

  assert.throws(() => ctv.verifyWebhook(body, "deadbeef"), CallToVerifyError);
  const tampered = "0" + sig.slice(1);
  assert.throws(() => ctv.verifyWebhook(body, tampered), CallToVerifyError);
});

test("constructor validates required options", () => {
  assert.throws(() => new CallToVerify({ apiKey: "k" }));
  assert.throws(() => new CallToVerify({ baseUrl: "http://x" }));
});
