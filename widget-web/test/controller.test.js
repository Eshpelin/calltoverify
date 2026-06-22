import test from "node:test";
import assert from "node:assert/strict";
import { createController } from "../dist/controller.js";

const instr = {
  number: "+8801700000001",
  code: "123456",
  channel: "sms",
  action: "Send 123456 to +8801700000001",
  deepLink: "sms:+8801700000001?body=123456",
  expiresAt: "2026-01-01T00:00:00Z",
};

test("begin loads instructions and moves to pending", async () => {
  const ctrl = createController({
    start: async () => ({ sessionId: "s1", instructions: instr }),
    status: async () => ({ status: "pending" }),
  });
  let got;
  ctrl.on("instructions", (i) => (got = i));
  await ctrl.begin();
  assert.equal(ctrl.state(), "pending");
  assert.equal(ctrl.sessionId(), "s1");
  assert.equal(got.code, "123456");
});

test("poll transitions to verified and emits the number", async () => {
  let n = 0;
  const ctrl = createController({
    start: async () => ({ sessionId: "s1", instructions: instr }),
    status: async () => (++n < 2 ? { status: "pending" } : { status: "verified", verifiedMsisdn: "+8801711111111" }),
  });
  await ctrl.begin();
  let verified;
  ctrl.on("verified", (m) => (verified = m));

  assert.equal(await ctrl.poll(), "pending");
  assert.equal(await ctrl.poll(), "verified");
  assert.equal(verified, "+8801711111111");
  assert.equal(ctrl.verifiedMsisdn(), "+8801711111111");

  // Further polls are no-ops once terminal.
  assert.equal(await ctrl.poll(), "verified");
});

test("expired status terminates", async () => {
  const ctrl = createController({
    start: async () => ({ sessionId: "s1", instructions: instr }),
    status: async () => ({ status: "expired" }),
  });
  await ctrl.begin();
  let expired = false;
  ctrl.on("expired", () => (expired = true));
  assert.equal(await ctrl.poll(), "expired");
  assert.ok(expired);
});

test("a failing start sets error state", async () => {
  const ctrl = createController({
    start: async () => {
      throw new Error("network");
    },
    status: async () => ({ status: "pending" }),
  });
  let errored = false;
  ctrl.on("error", () => (errored = true));
  await ctrl.begin();
  assert.equal(ctrl.state(), "error");
  assert.ok(errored);
});

test("a transient poll failure stays pending", async () => {
  let calls = 0;
  const ctrl = createController({
    start: async () => ({ sessionId: "s1", instructions: instr }),
    status: async () => {
      calls++;
      if (calls === 1) throw new Error("timeout");
      return { status: "pending" };
    },
  });
  await ctrl.begin();
  assert.equal(await ctrl.poll(), "pending"); // threw, still pending
  assert.equal(await ctrl.poll(), "pending");
});

test("begin passes the chosen channel to start", async () => {
  let seen;
  const ctrl = createController({
    start: async (channel) => {
      seen = channel;
      return { sessionId: "s1", instructions: instr };
    },
    status: async () => ({ status: "pending" }),
  });
  await ctrl.begin("call");
  assert.equal(seen, "call");
});

test("reset returns the controller to idle and re-emits", async () => {
  const ctrl = createController({
    start: async () => ({ sessionId: "s1", instructions: instr }),
    status: async () => ({ status: "pending" }),
  });
  await ctrl.begin("sms");
  assert.equal(ctrl.state(), "pending");

  let reset = false;
  ctrl.on("reset", () => (reset = true));
  ctrl.reset();
  assert.equal(ctrl.state(), "idle");
  assert.equal(ctrl.sessionId(), undefined);
  assert.ok(reset);

  // Can start again after reset.
  await ctrl.begin("call");
  assert.equal(ctrl.state(), "pending");
});
