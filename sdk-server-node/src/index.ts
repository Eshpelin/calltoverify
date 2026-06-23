/**
 * CallToVerify server SDK.
 *
 * Thin, dependency-free client for the Coordinator's developer API. Use it from
 * your backend to start verifications, poll their status, and verify webhooks.
 */
import { createHmac, timingSafeEqual } from "node:crypto";

export type Channel = "sms" | "call" | "dtmf";
export type BindingMode = "derive" | "claim";
export type VerificationState = "pending" | "verified" | "expired" | "failed";

export interface CallToVerifyOptions {
  /** Base URL of the Coordinator, e.g. https://verify.example.com */
  baseUrl: string;
  /** Developer API key (Bearer). */
  apiKey: string;
  /** Webhook signing secret; required only to call verifyWebhook. */
  webhookSecret?: string;
  /** Override the fetch implementation (defaults to global fetch, Node >= 18). */
  fetch?: typeof fetch;
  /** Per-request timeout in milliseconds (default 10000). */
  timeoutMs?: number;
}

export interface StartVerificationParams {
  channel?: Channel;
  bindingMode?: BindingMode;
  /** Required when bindingMode is "claim". */
  claimedMsisdn?: string;
}

export interface Instructions {
  number: string;
  code?: string;
  channel: Channel;
  action: string;
  deepLink: string;
  expiresAt: string;
}

export interface Verification {
  sessionId: string;
  status: VerificationState;
  instructions: Instructions;
}

export interface VerificationStatus {
  sessionId: string;
  status: VerificationState;
  channel: Channel;
  verifiedMsisdn?: string;
  expiresAt: string;
}

export interface WebhookEvent {
  event: string;
  sessionId: string;
  verifiedMsisdn: string;
  channel: Channel;
  ts: string;
}

/** Error thrown for non-2xx Coordinator responses and signature failures. */
export class CallToVerifyError extends Error {
  readonly status: number;
  readonly code: string;
  constructor(status: number, code: string, detail: string) {
    super(detail);
    this.name = "CallToVerifyError";
    this.status = status;
    this.code = code;
  }
}

export class CallToVerify {
  readonly #baseUrl: string;
  readonly #apiKey: string;
  readonly #webhookSecret?: string;
  readonly #fetch: typeof fetch;
  readonly #timeoutMs: number;

  constructor(opts: CallToVerifyOptions) {
    if (!opts?.baseUrl) throw new Error("CallToVerify: baseUrl is required");
    if (!opts?.apiKey) throw new Error("CallToVerify: apiKey is required");
    this.#baseUrl = opts.baseUrl.replace(/\/+$/, "");
    this.#apiKey = opts.apiKey;
    this.#webhookSecret = opts.webhookSecret;
    this.#timeoutMs = opts.timeoutMs ?? 10000;
    const f = opts.fetch ?? globalThis.fetch;
    if (!f) throw new Error("CallToVerify: no fetch available; pass opts.fetch (Node >= 18 has global fetch)");
    this.#fetch = f;
  }

  /** Start a verification and return the user-facing instructions. */
  async startVerification(params: StartVerificationParams = {}): Promise<Verification> {
    const body = JSON.stringify({
      channel: params.channel,
      binding_mode: params.bindingMode,
      claimed_msisdn: params.claimedMsisdn,
    });
    const data = await this.#request("POST", "/v1/verifications", body);
    return {
      sessionId: data.session_id,
      status: data.status,
      instructions: mapInstructions(data.instructions),
    };
  }

  /** Poll a verification's current status. */
  async checkStatus(sessionId: string): Promise<VerificationStatus> {
    const data = await this.#request("GET", `/v1/verifications/${encodeURIComponent(sessionId)}`);
    return {
      sessionId: data.session_id,
      status: data.status,
      channel: data.channel,
      verifiedMsisdn: data.verified_msisdn || undefined,
      expiresAt: data.expires_at,
    };
  }

  /**
   * Verify and parse a webhook. Pass the raw request body (string or Buffer) and
   * the X-CTV-Signature header. Throws CallToVerifyError on signature mismatch.
   *
   * Pass maxAgeSeconds to also reject events whose `ts` is outside that window
   * (replay defense). Still de-dupe on sessionId in your handler for idempotency.
   */
  verifyWebhook(rawBody: string | Buffer, signature: string, maxAgeSeconds?: number): WebhookEvent {
    if (!this.#webhookSecret) throw new Error("CallToVerify: webhookSecret is required to verify webhooks");
    // Fail closed (not TypeError) when the header is missing — Express returns
    // `string | string[] | undefined` for headers.
    if (typeof signature !== "string" || signature.length === 0) {
      throw new CallToVerifyError(401, "invalid_signature", "missing webhook signature");
    }
    const expected = createHmac("sha256", this.#webhookSecret).update(rawBody).digest("hex");
    const ok =
      signature.length === expected.length &&
      timingSafeEqual(Buffer.from(signature), Buffer.from(expected));
    if (!ok) throw new CallToVerifyError(401, "invalid_signature", "webhook signature mismatch");
    const p = JSON.parse(typeof rawBody === "string" ? rawBody : rawBody.toString("utf8"));
    if (maxAgeSeconds !== undefined) {
      const ts = Date.parse(p.ts);
      if (Number.isNaN(ts) || Math.abs(Date.now() - ts) > maxAgeSeconds * 1000) {
        throw new CallToVerifyError(401, "webhook_expired", "webhook timestamp outside the allowed window");
      }
    }
    // Coerce missing fields to "" (matches the Python/PHP SDKs) rather than
    // returning `undefined` typed as `string`.
    return {
      event: p.event ?? "",
      sessionId: p.session_id ?? "",
      verifiedMsisdn: p.verified_msisdn ?? "",
      channel: p.channel ?? "",
      ts: p.ts ?? "",
    };
  }

  async #request(method: string, path: string, body?: string): Promise<any> {
    let res: Response;
    try {
      res = await this.#fetch(this.#baseUrl + path, {
        method,
        headers: {
          Authorization: `Bearer ${this.#apiKey}`,
          ...(body ? { "Content-Type": "application/json" } : {}),
        },
        body,
        signal: AbortSignal.timeout(this.#timeoutMs),
      });
    } catch (e) {
      const name = (e as { name?: string })?.name;
      if (name === "TimeoutError" || name === "AbortError") {
        throw new CallToVerifyError(0, "timeout", `request timed out after ${this.#timeoutMs}ms`);
      }
      throw new CallToVerifyError(0, "network_error", (e as Error)?.message ?? "network error");
    }
    const text = await res.text();
    let json: any = {};
    if (text) {
      try {
        json = JSON.parse(text);
      } catch {
        // A non-JSON body (e.g. an HTML 502 from a proxy) must still surface as a
        // CallToVerifyError carrying the status, not a bare SyntaxError.
        throw new CallToVerifyError(res.status, res.ok ? "invalid_response" : "error", res.statusText || "non-JSON response body");
      }
    }
    if (!res.ok) {
      throw new CallToVerifyError(res.status, json.error ?? "error", json.detail ?? res.statusText);
    }
    return json;
  }
}

function mapInstructions(i: any): Instructions {
  return {
    number: i.number,
    code: i.code || undefined,
    channel: i.channel,
    action: i.action,
    deepLink: i.deep_link,
    expiresAt: i.expires_at,
  };
}
