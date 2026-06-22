/**
 * Framework-agnostic verification controller: a small state machine that drives a
 * verification from start through polling to a terminal state. It holds no DOM and
 * is fully unit-testable. The `mount` helper in index.ts wires it to the DOM.
 *
 * The controller talks to YOUR backend (via the injected start/status callbacks),
 * not to the Coordinator directly, so your API key never reaches the browser.
 */

export type VerificationState = "idle" | "pending" | "verified" | "expired" | "error";

export interface Instructions {
  number: string;
  code?: string;
  channel: string;
  action: string;
  deepLink: string;
  expiresAt: string;
}

export interface StartResult {
  sessionId: string;
  instructions: Instructions;
}

export interface StatusResult {
  status: string;
  verifiedMsisdn?: string;
}

export interface ControllerOptions {
  /** Begin a verification (proxied through your backend). */
  start: () => Promise<StartResult>;
  /** Fetch the current status for a session (proxied through your backend). */
  status: (sessionId: string) => Promise<StatusResult>;
}

type Listener = (payload?: unknown) => void;

export interface Controller {
  begin(): Promise<void>;
  poll(): Promise<VerificationState>;
  state(): VerificationState;
  sessionId(): string | undefined;
  instructions(): Instructions | undefined;
  verifiedMsisdn(): string | undefined;
  on(event: "instructions" | "verified" | "expired" | "error", cb: Listener): void;
}

export function createController(opts: ControllerOptions): Controller {
  let state: VerificationState = "idle";
  let sessionId: string | undefined;
  let instr: Instructions | undefined;
  let verified: string | undefined;
  const listeners: Record<string, Listener[]> = {};

  const emit = (event: string, payload?: unknown) => {
    (listeners[event] ?? []).forEach((fn) => fn(payload));
  };

  return {
    on(event, cb) {
      (listeners[event] ??= []).push(cb);
    },

    async begin() {
      if (state !== "idle") return;
      try {
        const r = await opts.start();
        sessionId = r.sessionId;
        instr = r.instructions;
        state = "pending";
        emit("instructions", instr);
      } catch (err) {
        state = "error";
        emit("error", err);
      }
    },

    async poll() {
      if (state !== "pending" || !sessionId) return state;
      try {
        const s = await opts.status(sessionId);
        if (s.status === "verified") {
          state = "verified";
          verified = s.verifiedMsisdn;
          emit("verified", verified);
        } else if (s.status === "expired" || s.status === "failed") {
          state = "expired";
          emit("expired");
        }
      } catch (err) {
        // Treat a failed poll as transient: stay pending and surface the error.
        emit("error", err);
      }
      return state;
    },

    state: () => state,
    sessionId: () => sessionId,
    instructions: () => instr,
    verifiedMsisdn: () => verified,
  };
}
