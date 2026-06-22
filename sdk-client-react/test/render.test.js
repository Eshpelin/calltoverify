import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { CallToVerify } from "../dist/index.js";

const props = {
  start: async () => ({ sessionId: "s", instructions: {} }),
  status: async () => ({ status: "pending" }),
};

test("renders the chooser for multiple channels", () => {
  const html = renderToStaticMarkup(React.createElement(CallToVerify, { ...props, channels: ["sms", "call"] }));
  assert.match(html, /Verify your number/);
  assert.match(html, /Text us a code/);
  assert.match(html, /Give a missed call/);
  assert.match(html, /Ring once and hang up/);
});

test("renders the starting state for a single channel", () => {
  const html = renderToStaticMarkup(React.createElement(CallToVerify, { ...props, channels: ["sms"] }));
  assert.match(html, /Starting/);
});

test("applies theme css variables to the root", () => {
  const html = renderToStaticMarkup(
    React.createElement(CallToVerify, { ...props, channels: ["sms", "call"], theme: { "--ctv-brand": "#0ea5e9" } }),
  );
  assert.match(html, /--ctv-brand:\s*#0ea5e9/);
});

test("custom labels override the defaults", () => {
  const html = renderToStaticMarkup(
    React.createElement(CallToVerify, { ...props, channels: ["sms", "call"], labels: { title: "Verifica tu número" } }),
  );
  assert.match(html, /Verifica tu n/);
});
