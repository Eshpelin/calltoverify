export type Channel = "sms" | "call" | "dtmf";

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
