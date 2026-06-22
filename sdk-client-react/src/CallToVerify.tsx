/**
 * CallToVerify React component. Renders the full multi-channel verification UX
 * (chooser, per-channel instructions, waiting/countdown, success/expiry) and talks
 * to YOUR backend via the `start` / `status` props, so the API key stays server-side.
 */
import { useCallback, useEffect, useRef, useState, type CSSProperties, type ReactElement } from "react";
import type { Channel, Instructions, StartResult, StatusResult } from "./types.js";
import { defaultLabels, type Labels } from "./i18n.js";
import { injectStyles } from "./styles.js";

export type { Channel } from "./types.js";
export type { Labels } from "./i18n.js";

export interface CallToVerifyProps {
  /** Channels to offer. With more than one, a chooser is shown first. */
  channels?: Channel[];
  start: (channel?: Channel) => Promise<StartResult>;
  status: (sessionId: string) => Promise<StatusResult>;
  onVerified?: (verifiedMsisdn?: string) => void;
  onExpired?: () => void;
  pollIntervalMs?: number;
  labels?: Partial<Labels>;
  /** Override --ctv-* CSS variables, e.g. { "--ctv-brand": "#0ea5e9" }. */
  theme?: Record<string, string>;
  className?: string;
}

type Phase = "choose" | "starting" | "instr" | "verified" | "expired" | "error";

export function CallToVerify(props: CallToVerifyProps): ReactElement {
  const { channels = [], start, status, onVerified, onExpired, pollIntervalMs = 2500, theme } = props;
  const L: Labels = { ...defaultLabels, ...props.labels };

  const [phase, setPhase] = useState<Phase>(channels.length > 1 ? "choose" : "starting");
  const [instr, setInstr] = useState<Instructions | null>(null);
  const [verified, setVerified] = useState<string | undefined>(undefined);
  const [remaining, setRemaining] = useState(0);
  const sessionRef = useRef<string | undefined>(undefined);
  const startedRef = useRef(false);

  useEffect(() => injectStyles(), []);

  const begin = useCallback(
    (channel?: Channel) => {
      setInstr(null);
      sessionRef.current = undefined;
      setPhase("starting");
      start(channel)
        .then((r) => {
          sessionRef.current = r.sessionId;
          setInstr(r.instructions);
          setPhase("instr");
        })
        .catch(() => setPhase("error"));
    },
    [start],
  );

  useEffect(() => {
    if (channels.length <= 1 && !startedRef.current) {
      startedRef.current = true;
      begin(channels[0]);
    }
  }, [begin, channels]);

  useEffect(() => {
    if (phase !== "instr") return;
    const id = setInterval(() => {
      const sid = sessionRef.current;
      if (!sid) return;
      status(sid)
        .then((s) => {
          if (s.status === "verified") {
            setVerified(s.verifiedMsisdn);
            setPhase("verified");
            onVerified?.(s.verifiedMsisdn);
          } else if (s.status === "expired" || s.status === "failed") {
            setPhase("expired");
            onExpired?.();
          }
        })
        .catch(() => {});
    }, pollIntervalMs);
    return () => clearInterval(id);
  }, [phase, status, pollIntervalMs, onVerified, onExpired]);

  useEffect(() => {
    if (phase !== "instr" || !instr) return;
    const tick = () => setRemaining(Math.max(0, Math.floor((Date.parse(instr.expiresAt) - Date.now()) / 1000)));
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [phase, instr]);

  const retry = () => (channels.length > 1 ? setPhase("choose") : begin(channels[0]));
  const cls = "ctv-widget" + (props.className ? " " + props.className : "");

  return (
    <div className={cls} style={theme as unknown as CSSProperties | undefined}>
      {phase === "choose" && <Chooser channels={channels} L={L} onPick={begin} />}
      {phase === "starting" && <p className="ctv-action">{L.startingText}</p>}
      {phase === "instr" && instr && (
        <Instr instr={instr} L={L} remaining={remaining} canSwitch={channels.length > 1} onSwitch={() => setPhase("choose")} />
      )}
      {phase === "verified" && <Final kind="ok" title={L.successTitle} sub={L.successSubtitle} icon={<IconCheck />} num={verified} />}
      {phase === "expired" && <Final kind="bad" title={L.expiredTitle} icon={<IconAlert />} retry={L.expiredRetry} onRetry={retry} />}
      {phase === "error" && <Final kind="bad" title={L.errorTitle} icon={<IconAlert />} retry={L.errorRetry} onRetry={retry} />}
    </div>
  );
}

function Chooser({ channels, L, onPick }: { channels: Channel[]; L: Labels; onPick: (c: Channel) => void }): ReactElement {
  const meta: Record<Channel, { icon: ReactElement; main: string; hint: string; free?: boolean }> = {
    sms: { icon: <IconSms />, main: L.chooseSms, hint: L.chooseSmsHint },
    call: { icon: <IconCall />, main: L.chooseCall, hint: L.chooseCallHint, free: true },
    dtmf: { icon: <IconDtmf />, main: L.chooseDtmf, hint: L.chooseDtmfHint },
  };
  return (
    <>
      <p className="ctv-title">{L.title}</p>
      <p className="ctv-subtitle">{L.subtitle}</p>
      <div className="ctv-choices">
        {channels.map((c) => (
          <button key={c} className="ctv-choice" onClick={() => onPick(c)} type="button">
            <span className="ctv-choice-ic">{meta[c].icon}</span>
            <span>
              <span className="ctv-choice-main">{meta[c].main}</span>
              <br />
              <span className={"ctv-choice-hint" + (meta[c].free ? " free" : "")}>{meta[c].hint}</span>
            </span>
          </button>
        ))}
      </div>
    </>
  );
}

function Instr(props: {
  instr: Instructions;
  L: Labels;
  remaining: number;
  canSwitch: boolean;
  onSwitch: () => void;
}): ReactElement {
  const { instr, L, remaining, canSwitch, onSwitch } = props;
  const waiting = (
    <>
      <div className="ctv-wait">
        <span className="ctv-spinner" />
        {L.waiting}
      </div>
      <div className="ctv-meta">
        <span>{L.expiresIn(remaining)}</span>
        {canSwitch ? (
          <button className="ctv-btn-ghost" type="button" onClick={onSwitch}>
            {L.tryAnother}
          </button>
        ) : (
          <span />
        )}
      </div>
    </>
  );

  if (instr.channel === "sms") {
    return (
      <>
        <p className="ctv-action">{L.smsAction(instr.number)}</p>
        <div className="ctv-code">
          {(instr.code ?? "").split("").map((d, i) => (
            <span key={i} className="ctv-digit">
              {d}
            </span>
          ))}
        </div>
        <a className="ctv-btn" href={instr.deepLink}>
          <IconSms />
          {L.smsButton}
        </a>
        {waiting}
      </>
    );
  }
  if (instr.channel === "call") {
    return (
      <>
        <p className="ctv-action">{L.callAction(instr.number)}</p>
        <p className="ctv-num">{instr.number}</p>
        <div className="ctv-steps">
          <div className="ctv-step">
            <span className="ctv-step-n">1</span>
            {L.callStep1}
          </div>
          <div className="ctv-step">
            <span className="ctv-step-n">2</span>
            {L.callStep2}
          </div>
          <div className="ctv-step">
            <span className="ctv-step-n">3</span>
            {L.callStep3}
          </div>
        </div>
        <a className="ctv-btn" href={instr.deepLink}>
          <IconCall />
          {L.callButton}
        </a>
        <div className="ctv-trust">
          <IconShield />
          {L.callTrust}
        </div>
        {waiting}
      </>
    );
  }
  return (
    <>
      <p className="ctv-action">{L.dtmfAction(instr.number)}</p>
      <div className="ctv-code">
        {(instr.code ?? "").split("").map((d, i) => (
          <span key={i} className="ctv-digit">
            {d}
          </span>
        ))}
      </div>
      <a className="ctv-btn" href={instr.deepLink}>
        <IconCall />
        {L.dtmfButton}
      </a>
      {waiting}
    </>
  );
}

function Final(props: {
  kind: "ok" | "bad";
  title: string;
  sub?: string;
  icon: ReactElement;
  num?: string;
  retry?: string;
  onRetry?: () => void;
}): ReactElement {
  return (
    <div className="ctv-final">
      <div className={"ctv-badge " + props.kind}>{props.icon}</div>
      <p className="ctv-final-title">{props.title}</p>
      {props.num && <p className="ctv-final-num">{props.num}</p>}
      {props.sub && <p className="ctv-final-sub">{props.sub}</p>}
      {props.retry && (
        <button className="ctv-btn" type="button" onClick={props.onRetry}>
          {props.retry}
        </button>
      )}
    </div>
  );
}

const svg = { fill: "none", stroke: "currentColor", strokeWidth: 2, strokeLinecap: "round", strokeLinejoin: "round" } as const;

function IconSms(): ReactElement {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" {...svg}>
      <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />
    </svg>
  );
}
function IconCall(): ReactElement {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" {...svg}>
      <path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72c.13.96.36 1.9.7 2.81a2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45c.91.34 1.85.57 2.81.7A2 2 0 0 1 22 16.92z" />
    </svg>
  );
}
function IconDtmf(): ReactElement {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" {...svg}>
      <circle cx="5" cy="6" r="1" />
      <circle cx="12" cy="6" r="1" />
      <circle cx="19" cy="6" r="1" />
      <circle cx="5" cy="12" r="1" />
      <circle cx="12" cy="12" r="1" />
      <circle cx="19" cy="12" r="1" />
      <circle cx="12" cy="18" r="1" />
    </svg>
  );
}
function IconCheck(): ReactElement {
  return (
    <svg viewBox="0 0 24 24" width="32" height="32" {...svg} strokeWidth={2.5}>
      <path d="M20 6 9 17l-5-5" />
    </svg>
  );
}
function IconAlert(): ReactElement {
  return (
    <svg viewBox="0 0 24 24" width="30" height="30" {...svg}>
      <circle cx="12" cy="12" r="10" />
      <path d="M12 8v4M12 16h.01" />
    </svg>
  );
}
function IconShield(): ReactElement {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" {...svg}>
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    </svg>
  );
}
