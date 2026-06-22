# receiver-pi

CallToVerify receiver for a Raspberry Pi (or any Linux box) with a GSM modem. It supports all
three channels, including DTMF, which the Android receiver cannot do. It reports to the
developer's backend using the same signed device protocol as the Android app.

> **Status: alpha.** The client, signing, and retry buffer are unit-tested here. The gammu and
> Asterisk integration is glue that needs real hardware to run, so it is shipped as reviewed
> configuration + scripts (not executed in CI).

## What it does

| Channel | How |
|---|---|
| **SMS** | `gammu-smsd` receives the SMS and calls `ctv-pi on-sms` (reads `SMS_1_NUMBER` / `SMS_1_TEXT`). |
| **Missed call** | Asterisk dialplan captures the caller id, hangs up, and runs `ctv-pi on-call`. |
| **DTMF** | Asterisk answers and an AGI script (`agi/ctv_dtmf.py`) collects the digits. |

A `ctv-pi run` daemon keeps the device online with heartbeats and drains a durable retry buffer.

## Install

```bash
cd receiver-pi
pip install -e .        # provides the `ctv-pi` command
```

## Pair and register

A Pi has no camera, so it doesn't scan the pairing QR. In the console, choose **Add a device →
Raspberry Pi** and it gives you a ready-to-paste `ctv-pi pair` command (the same credentials the QR
would encode): `{"endpoint":"https://your-backend/ctv","device_id":"...","device_secret":"..."}`.

```bash
ctv-pi pair '{"endpoint":"https://your-backend/ctv","device_id":"...","device_secret":"..."}' \
  --msisdn "+8801700000001"
ctv-pi register     # announces the device; caches its number if not set
```

## SMS channel (gammu)

```bash
sudo apt install gammu gammu-smsd
sudo cp contrib/gammu-smsd.conf.example /etc/gammu-smsd.conf   # edit device/connection
sudo systemctl enable --now gammu-smsd
```

`RunOnReceive = ctv-pi on-sms` posts each inbound SMS to your backend.

## Missed-call and DTMF channels (Asterisk)

Install Asterisk with a GSM channel driver (for example `chan_dongle`), then add the dialplan in
`contrib/asterisk-extensions.conf.example` and copy `agi/ctv_dtmf.py` to the path it references.

### DTMF voice prompt (optional)

On a DTMF call the AGI plays a prompt, then collects the digits. By default it just plays a `beep`.
To voice-guide the caller instead ("please enter the code shown on your screen, then press the pound
key"), generate the prompt and point the receiver at it:

```bash
sudo apt install espeak-ng sox
./contrib/make-dtmf-prompt.sh                       # writes ctv-enter-pin.wav (8 kHz mono)
sudo cp ctv-enter-pin.wav /usr/share/asterisk/sounds/en/
export CTV_DTMF_PROMPT=ctv-enter-pin                # or set "dtmf_prompt" in config.json
```

Any Asterisk sound name works as `CTV_DTMF_PROMPT`; record your own prompt if you prefer.

## Run the heartbeat daemon

```bash
sudo cp contrib/ctv-pi.service /etc/systemd/system/
sudo systemctl enable --now ctv-pi
```

## Configuration

Read from `~/.config/calltoverify/config.json` (written by `ctv-pi pair`), overridable by env:
`CTV_ENDPOINT`, `CTV_DEVICE_ID`, `CTV_DEVICE_SECRET`, `CTV_MSISDN`, `CTV_QUEUE`.

## Develop

```bash
python -m unittest discover -s tests -t .
```

Tests cover the signed client (matching the Go Coordinator's HMAC scheme) and the retry buffer.
