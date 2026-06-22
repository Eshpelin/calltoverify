/**
 * CallToVerify embeddable web widget.
 *
 * `mount` renders the verification instructions (with a tap-to-send deep link and a
 * countdown), polls status through your backend, and flips to a success state when
 * the number is verified.
 */
import { createController, type ControllerOptions, type Instructions } from "./controller.js";

export * from "./controller.js";

export interface MountOptions extends ControllerOptions {
  /** Poll interval in milliseconds (default 2500). */
  pollIntervalMs?: number;
  /** Called when verification succeeds. */
  onVerified?: (verifiedMsisdn?: string) => void;
  /** Called when the session expires or fails. */
  onExpired?: () => void;
  /** Override the button label (default "Open"). */
  openLabel?: string;
}

export interface WidgetHandle {
  stop(): void;
  controller: ReturnType<typeof createController>;
}

export function mount(target: HTMLElement | string, opts: MountOptions): WidgetHandle {
  const el = typeof target === "string" ? document.querySelector(target) : target;
  if (!el) throw new Error("CallToVerify widget: target element not found");

  const root = document.createElement("div");
  root.className = "ctv-widget";
  root.innerHTML = `<p class="ctv-status">Starting…</p>`;
  el.innerHTML = "";
  el.appendChild(root);

  const ctrl = createController(opts);
  let pollTimer: ReturnType<typeof setInterval> | undefined;
  let countdownTimer: ReturnType<typeof setInterval> | undefined;

  const stop = () => {
    if (pollTimer) clearInterval(pollTimer);
    if (countdownTimer) clearInterval(countdownTimer);
    pollTimer = countdownTimer = undefined;
  };

  ctrl.on("instructions", (i) => {
    const instr = i as Instructions;
    renderInstructions(root, instr, opts.openLabel ?? "Open");
    countdownTimer = setInterval(() => updateCountdown(root, instr.expiresAt), 1000);
    updateCountdown(root, instr.expiresAt);
  });

  ctrl.on("verified", (m) => {
    stop();
    const who = typeof m === "string" && m ? `: ${escapeHtml(m)}` : "";
    root.innerHTML = `<p class="ctv-status ctv-success">Verified${who}</p>`;
    opts.onVerified?.(m as string | undefined);
  });

  ctrl.on("expired", () => {
    stop();
    root.innerHTML = `<p class="ctv-status ctv-expired">This code expired. Please try again.</p>`;
    opts.onExpired?.();
  });

  void ctrl.begin().then(() => {
    if (ctrl.state() === "pending") {
      pollTimer = setInterval(() => void ctrl.poll(), opts.pollIntervalMs ?? 2500);
    } else if (ctrl.state() === "error") {
      root.innerHTML = `<p class="ctv-status ctv-error">Could not start verification. Please retry.</p>`;
    }
  });

  return { stop, controller: ctrl };
}

function renderInstructions(root: HTMLElement, i: Instructions, openLabel: string): void {
  root.innerHTML = `
    <p class="ctv-action">${escapeHtml(i.action)}</p>
    <a class="ctv-button" href="${escapeAttr(i.deepLink)}">${escapeHtml(openLabel)}</a>
    <p class="ctv-countdown" data-expires="${escapeAttr(i.expiresAt)}"></p>
    <p class="ctv-status">Waiting for your message…</p>
  `;
}

function updateCountdown(root: HTMLElement, expiresAt: string): void {
  const el = root.querySelector(".ctv-countdown");
  if (!el) return;
  const remaining = Math.max(0, Math.floor((Date.parse(expiresAt) - Date.now()) / 1000));
  el.textContent = remaining > 0 ? `Expires in ${remaining}s` : "Expired";
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!);
}

function escapeAttr(s: string): string {
  return escapeHtml(s);
}
