/** User-facing copy. Override any field via the `labels` prop to translate or
 *  re-voice the component. */
export interface Labels {
  title: string;
  subtitle: string;
  chooseSms: string;
  chooseSmsHint: string;
  chooseCall: string;
  chooseCallHint: string;
  chooseDtmf: string;
  chooseDtmfHint: string;
  smsAction: (number: string) => string;
  smsButton: string;
  callAction: (number: string) => string;
  callStep1: string;
  callStep2: string;
  callStep3: string;
  callTrust: string;
  callButton: string;
  dtmfAction: (number: string, code: string) => string;
  dtmfButton: string;
  waiting: string;
  expiresIn: (seconds: number) => string;
  tryAnother: string;
  successTitle: string;
  successSubtitle: string;
  expiredTitle: string;
  expiredRetry: string;
  errorTitle: string;
  errorRetry: string;
  startingText: string;
}

export const defaultLabels: Labels = {
  title: "Verify your number",
  subtitle: "We don't send you a code. You contact us from your number instead, and we verify it.",
  chooseSms: "Text us a code",
  chooseSmsHint: "Send one text message",
  chooseCall: "Give a missed call",
  chooseCallHint: "Ring once and hang up",
  chooseDtmf: "Call and enter a code",
  chooseDtmfHint: "Type it on the keypad",
  smsAction: (number) => `Send the code below to ${number}`,
  smsButton: "Open messages",
  callAction: (number) => `Give a quick missed call to ${number}`,
  callStep1: "Tap call",
  callStep2: "Let it ring once",
  callStep3: "Hang up. Done.",
  callTrust: "We only see your number — we don't pick up.",
  callButton: "Call now",
  dtmfAction: (number, code) => `Call ${number} and enter ${code} on the keypad`,
  dtmfButton: "Call now",
  waiting: "Waiting for you. We detect it automatically.",
  expiresIn: (s) => `Expires in ${s}s`,
  tryAnother: "Try another way",
  successTitle: "Your number is verified",
  successSubtitle: "",
  expiredTitle: "That code expired",
  expiredRetry: "Try again",
  errorTitle: "Something went wrong",
  errorRetry: "Retry",
  startingText: "Starting…",
};
