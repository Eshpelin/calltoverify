/** The shared CallToVerify look (bold, branded, themeable via --ctv-* variables).
 *  Kept in sync with @calltoverify/widget. */
export const css = `
.ctv-widget {
  --ctv-brand: #4f46e5;
  --ctv-brand-strong: #4338ca;
  --ctv-on-brand: #ffffff;
  --ctv-bg: #ffffff;
  --ctv-surface: #f4f4fb;
  --ctv-text: #17172b;
  --ctv-muted: #6b6b82;
  --ctv-success: #15a34a;
  --ctv-danger: #dc2626;
  --ctv-border: rgba(23,23,43,0.10);
  --ctv-radius: 18px;
  --ctv-radius-sm: 11px;
  --ctv-font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;

  box-sizing: border-box;
  font-family: var(--ctv-font);
  color: var(--ctv-text);
  background: var(--ctv-bg);
  border: 1px solid var(--ctv-border);
  border-radius: var(--ctv-radius);
  padding: 22px;
  max-width: 380px;
  width: 100%;
  line-height: 1.5;
}
.ctv-widget * { box-sizing: border-box; }
.ctv-title { font-size: 19px; font-weight: 700; margin: 0 0 4px; letter-spacing: -0.01em; }
.ctv-subtitle { font-size: 13.5px; color: var(--ctv-muted); margin: 0 0 18px; }
.ctv-choices { display: flex; flex-direction: column; gap: 10px; }
.ctv-choice {
  display: flex; align-items: center; gap: 13px; width: 100%; text-align: left;
  background: var(--ctv-bg); border: 1.5px solid var(--ctv-border);
  border-radius: var(--ctv-radius-sm); padding: 13px 15px; cursor: pointer;
  font-family: inherit; color: inherit; transition: border-color .15s, transform .05s;
}
.ctv-choice:hover { border-color: var(--ctv-brand); }
.ctv-choice:active { transform: scale(0.99); }
.ctv-choice-ic {
  width: 38px; height: 38px; flex: none; border-radius: 11px; display: flex;
  align-items: center; justify-content: center; background: var(--ctv-surface);
  color: var(--ctv-brand); font-size: 19px;
}
.ctv-choice-main { font-size: 14.5px; font-weight: 600; }
.ctv-choice-hint { font-size: 12px; color: var(--ctv-muted); }
.ctv-choice-hint.free { color: var(--ctv-success); font-weight: 600; }
.ctv-action { font-size: 14px; color: var(--ctv-muted); margin: 2px 0 16px; }
.ctv-code { display: flex; gap: 9px; justify-content: center; margin: 0 0 18px; }
.ctv-digit {
  width: 46px; height: 56px; display: flex; align-items: center; justify-content: center;
  font-size: 26px; font-weight: 700; color: var(--ctv-brand-strong);
  background: var(--ctv-surface); border-radius: 12px; font-variant-numeric: tabular-nums;
}
.ctv-num { font-size: 21px; font-weight: 700; text-align: center; margin: 2px 0 16px; }
.ctv-btn {
  display: flex; align-items: center; justify-content: center; gap: 9px; width: 100%;
  background: var(--ctv-brand); color: var(--ctv-on-brand); border: none;
  border-radius: var(--ctv-radius-sm); padding: 14px; font-size: 15px; font-weight: 600;
  font-family: inherit; cursor: pointer; text-decoration: none; transition: background .15s, transform .05s;
}
.ctv-btn:hover { background: var(--ctv-brand-strong); }
.ctv-btn:active { transform: scale(0.99); }
.ctv-btn-ghost {
  background: none; border: none; color: var(--ctv-brand);
  font-family: inherit; font-size: 13px; font-weight: 600; cursor: pointer; padding: 6px 0;
}
.ctv-steps { display: flex; flex-direction: column; gap: 9px; margin: 0 0 16px; }
.ctv-step { display: flex; align-items: center; gap: 10px; font-size: 13.5px; }
.ctv-step-n {
  width: 22px; height: 22px; flex: none; border-radius: 50%; background: var(--ctv-surface);
  color: var(--ctv-brand-strong); font-size: 12px; font-weight: 700;
  display: flex; align-items: center; justify-content: center;
}
.ctv-wait { display: flex; align-items: center; gap: 9px; margin-top: 16px; font-size: 12.5px; color: var(--ctv-muted); }
.ctv-spinner {
  width: 15px; height: 15px; border-radius: 50%; flex: none;
  border: 2px solid var(--ctv-border); border-top-color: var(--ctv-brand);
  animation: ctv-spin 0.8s linear infinite;
}
@keyframes ctv-spin { to { transform: rotate(360deg); } }
.ctv-meta { font-size: 11.5px; color: var(--ctv-muted); margin-top: 9px; display: flex; justify-content: space-between; align-items: center; }
.ctv-trust { display: flex; align-items: center; gap: 7px; font-size: 11.5px; color: var(--ctv-success); margin-top: 12px; }
.ctv-final { text-align: center; padding: 14px 0 6px; }
.ctv-badge {
  width: 60px; height: 60px; border-radius: 50%; margin: 0 auto 14px; display: flex;
  align-items: center; justify-content: center; font-size: 32px;
}
.ctv-badge.ok { background: rgba(21,163,74,0.12); color: var(--ctv-success); }
.ctv-badge.bad { background: rgba(220,38,38,0.10); color: var(--ctv-danger); }
.ctv-final-title { font-size: 18px; font-weight: 700; margin: 0 0 4px; }
.ctv-final-sub { font-size: 13px; color: var(--ctv-muted); margin: 0 0 16px; }
.ctv-final-num { font-size: 15px; font-weight: 600; margin: 0 0 12px; }
@media (prefers-color-scheme: dark) {
  .ctv-widget {
    --ctv-bg: #15151f; --ctv-surface: #20202e; --ctv-text: #f1f1f6;
    --ctv-muted: #9a9ab0; --ctv-border: rgba(255,255,255,0.12);
    --ctv-brand: #7c74ff; --ctv-brand-strong: #6a61f0;
  }
  .ctv-digit { color: #cfcaff; }
}
`;

/** Injects the stylesheet once. No-op during server rendering. */
export function injectStyles(): void {
  if (typeof document === "undefined") return;
  if (document.getElementById("ctv-widget-styles")) return;
  const el = document.createElement("style");
  el.id = "ctv-widget-styles";
  el.textContent = css;
  document.head.appendChild(el);
}
