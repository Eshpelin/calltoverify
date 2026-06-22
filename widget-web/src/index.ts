/**
 * CallToVerify embeddable web widget.
 *
 * `mount` renders the full end-user experience: an optional channel chooser, the
 * per-channel instruction (with a tap-to-send deep link and a live countdown),
 * status polling, and success / expiry states. It talks to YOUR backend via the
 * injected start/status callbacks, so your API key never reaches the browser.
 */
import { createController, type Channel, type ControllerOptions, type Instructions } from "./controller.js";
import { defaultLabels, type Labels } from "./i18n.js";
import { ensureStyles } from "./styles.js";

export * from "./controller.js";
export type { Labels } from "./i18n.js";

export interface MountOptions extends ControllerOptions {
  /** Channels to offer. If more than one, a chooser is shown first. Defaults to a
   *  single, backend-chosen channel (no chooser). */
  channels?: Channel[];
  pollIntervalMs?: number;
  onVerified?: (verifiedMsisdn?: string) => void;
  onExpired?: () => void;
  /** Override any user-facing string. */
  labels?: Partial<Labels>;
  /** Override --ctv-* CSS variables, e.g. { "--ctv-brand": "#0ea5e9" }. */
  theme?: Record<string, string>;
}

export interface WidgetHandle {
  stop(): void;
  controller: ReturnType<typeof createController>;
}

const ICONS: Record<string, string> = {
  sms: `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"/></svg>`,
  call: `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72c.13.96.36 1.9.7 2.81a2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45c.91.34 1.85.57 2.81.7A2 2 0 0 1 22 16.92z"/></svg>`,
  dtmf: `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="5" cy="6" r="1"/><circle cx="12" cy="6" r="1"/><circle cx="19" cy="6" r="1"/><circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="12" cy="18" r="1"/></svg>`,
  check: `<svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>`,
  alert: `<svg viewBox="0 0 24 24" width="30" height="30" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 8v4M12 16h.01"/></svg>`,
  shield: `<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>`,
};

export function mount(target: HTMLElement | string, opts: MountOptions): WidgetHandle {
  const el = typeof target === "string" ? document.querySelector(target) : target;
  if (!el) throw new Error("CallToVerify widget: target element not found");
  ensureStyles(document);

  const L: Labels = { ...defaultLabels, ...opts.labels };
  const channels = opts.channels ?? [];

  const root = document.createElement("div");
  root.className = "ctv-widget";
  for (const [k, v] of Object.entries(opts.theme ?? {})) root.style.setProperty(k, v);
  el.innerHTML = "";
  el.appendChild(root);

  const ctrl = createController(opts);
  let pollTimer: ReturnType<typeof setInterval> | undefined;
  let countdownTimer: ReturnType<typeof setInterval> | undefined;

  const stopTimers = () => {
    if (pollTimer) clearInterval(pollTimer);
    if (countdownTimer) clearInterval(countdownTimer);
    pollTimer = countdownTimer = undefined;
  };

  ctrl.on("instructions", (i) => {
    renderInstructions(root, i as Instructions, L, channels.length > 1, () => switchChannel());
    pollTimer = setInterval(() => void ctrl.poll(), opts.pollIntervalMs ?? 2500);
    const tick = () => updateCountdown(root, (i as Instructions).expiresAt, L);
    countdownTimer = setInterval(tick, 1000);
    tick();
  });

  ctrl.on("verified", (m) => {
    stopTimers();
    renderFinal(root, "ok", L.successTitle, L.successSubtitle, ICONS.check, typeof m === "string" ? m : undefined);
    opts.onVerified?.(m as string | undefined);
  });

  ctrl.on("expired", () => {
    stopTimers();
    renderFinal(root, "bad", L.expiredTitle, "", ICONS.alert, undefined, L.expiredRetry, () => restart());
    opts.onExpired?.();
  });

  ctrl.on("error", () => {
    if (ctrl.state() === "error") {
      stopTimers();
      renderFinal(root, "bad", L.errorTitle, "", ICONS.alert, undefined, L.errorRetry, () => restart());
    }
  });

  function begin(channel?: Channel) {
    root.innerHTML = `<p class="ctv-action">${escapeHtml(L.startingText)}</p>`;
    void ctrl.begin(channel);
  }

  function start() {
    if (channels.length > 1) {
      renderChooser(root, channels, L, (c) => begin(c));
    } else {
      begin(channels[0]);
    }
  }

  function switchChannel() {
    stopTimers();
    ctrl.reset();
    start();
  }

  function restart() {
    stopTimers();
    ctrl.reset();
    start();
  }

  start();
  return { stop: stopTimers, controller: ctrl };
}

function renderChooser(root: HTMLElement, channels: Channel[], L: Labels, onPick: (c: Channel) => void): void {
  const meta: Record<Channel, { ic: string; main: string; hint: string; free?: boolean }> = {
    sms: { ic: ICONS.sms, main: L.chooseSms, hint: L.chooseSmsHint },
    call: { ic: ICONS.call, main: L.chooseCall, hint: L.chooseCallHint, free: true },
    dtmf: { ic: ICONS.dtmf, main: L.chooseDtmf, hint: L.chooseDtmfHint },
  };
  const items = channels
    .map(
      (c) => `<button class="ctv-choice" data-ch="${c}">
        <span class="ctv-choice-ic">${meta[c].ic}</span>
        <span><span class="ctv-choice-main">${escapeHtml(meta[c].main)}</span><br>
        <span class="ctv-choice-hint${meta[c].free ? " free" : ""}">${escapeHtml(meta[c].hint)}</span></span>
      </button>`,
    )
    .join("");
  root.innerHTML = `<p class="ctv-title">${escapeHtml(L.title)}</p>
    <p class="ctv-subtitle">${escapeHtml(L.subtitle)}</p>
    <div class="ctv-choices">${items}</div>`;
  root.querySelectorAll<HTMLButtonElement>(".ctv-choice").forEach((b) => {
    b.addEventListener("click", () => onPick(b.dataset.ch as Channel));
  });
}

function renderInstructions(root: HTMLElement, i: Instructions, L: Labels, canSwitch: boolean, onSwitch: () => void): void {
  const waiting = `<div class="ctv-wait"><span class="ctv-spinner"></span>${escapeHtml(L.waiting)}</div>
    <div class="ctv-meta"><span class="ctv-countdown"></span>${
      canSwitch ? `<button class="ctv-btn-ghost" data-switch>${escapeHtml(L.tryAnother)}</button>` : "<span></span>"
    }</div>`;

  if (i.channel === "sms") {
    const digits = (i.code ?? "").split("").map((d) => `<span class="ctv-digit">${escapeHtml(d)}</span>`).join("");
    root.innerHTML = `<p class="ctv-action">${escapeHtml(L.smsAction(i.number))}</p>
      <div class="ctv-code">${digits}</div>
      <a class="ctv-btn" href="${escapeAttr(i.deepLink)}">${ICONS.sms}${escapeHtml(L.smsButton)}</a>
      ${waiting}`;
  } else if (i.channel === "call") {
    root.innerHTML = `<p class="ctv-action">${escapeHtml(L.callAction(i.number))}</p>
      <p class="ctv-num">${escapeHtml(i.number)}</p>
      <div class="ctv-steps">
        <div class="ctv-step"><span class="ctv-step-n">1</span>${escapeHtml(L.callStep1)}</div>
        <div class="ctv-step"><span class="ctv-step-n">2</span>${escapeHtml(L.callStep2)}</div>
        <div class="ctv-step"><span class="ctv-step-n">3</span>${escapeHtml(L.callStep3)}</div>
      </div>
      <a class="ctv-btn" href="${escapeAttr(i.deepLink)}">${ICONS.call}${escapeHtml(L.callButton)}</a>
      <div class="ctv-trust">${ICONS.shield}${escapeHtml(L.callTrust)}</div>
      ${waiting}`;
  } else {
    root.innerHTML = `<p class="ctv-action">${escapeHtml(L.dtmfAction(i.number, i.code ?? ""))}</p>
      <p class="ctv-num">${escapeHtml(i.number)}</p>
      <a class="ctv-btn" href="${escapeAttr(i.deepLink)}">${ICONS.call}${escapeHtml(L.dtmfButton)}</a>
      ${waiting}`;
  }

  const sw = root.querySelector<HTMLButtonElement>("[data-switch]");
  if (sw) sw.addEventListener("click", onSwitch);
}

function updateCountdown(root: HTMLElement, expiresAt: string, L: Labels): void {
  const el = root.querySelector(".ctv-countdown");
  if (!el) return;
  const remaining = Math.max(0, Math.floor((Date.parse(expiresAt) - Date.now()) / 1000));
  el.textContent = L.expiresIn(remaining);
}

function renderFinal(
  root: HTMLElement,
  kind: "ok" | "bad",
  title: string,
  subtitle: string,
  icon: string,
  number?: string,
  retryLabel?: string,
  onRetry?: () => void,
): void {
  root.innerHTML = `<div class="ctv-final">
    <div class="ctv-badge ${kind}">${icon}</div>
    <p class="ctv-final-title">${escapeHtml(title)}</p>
    ${number ? `<p class="ctv-final-num">${escapeHtml(number)}</p>` : ""}
    ${subtitle ? `<p class="ctv-final-sub">${escapeHtml(subtitle)}</p>` : ""}
    ${retryLabel ? `<button class="ctv-btn" data-retry>${escapeHtml(retryLabel)}</button>` : ""}
  </div>`;
  const r = root.querySelector<HTMLButtonElement>("[data-retry]");
  if (r && onRetry) r.addEventListener("click", onRetry);
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!);
}

function escapeAttr(s: string): string {
  return escapeHtml(s);
}
