# receiver-pi

CallToVerify receiver for a Raspberry Pi (or any Linux box) with a GSM modem. It's the most capable
receiver — it's the **only** one that can do DTMF — and it runs headless as an always-on service. It
reports to your backend with the same signed protocol as the Android app.

> **A phone is easier.** If you only need **SMS** and **missed call**, a spare Android phone
> ([download the app](https://github.com/Eshpelin/calltoverify/releases/latest/download/calltoverify-receiver.apk),
> install, scan a QR) is far less work. Use a Pi when you need **DTMF**, or want a dedicated headless box.

> **Status: alpha.** The client, signing, and retry buffer are unit-tested here. The gammu and
> Asterisk pieces are integration glue that needs real hardware, so they ship as reviewed
> configuration + scripts (not run in CI).

## What you need

- A Raspberry Pi (3 / 4 / Zero 2 W are all fine) running Raspberry Pi OS, or any Linux box.
- A **USB GSM modem or HAT with a SIM in it** — this is the part that takes effort. Make sure your
  modem is exposed as a serial device (`/dev/ttyUSB*`).
- Network access from the Pi to your CallToVerify backend.

## What each channel needs

Set up only what you need. Start with SMS; add Asterisk later only if you want voice.

| Channel | Needs | Effort |
|---|---|---|
| **SMS** | `gammu-smsd` | easy |
| **Missed call** | Asterisk + a GSM channel driver (e.g. `chan_dongle`) | involved |
| **DTMF** | Asterisk (same as missed call) | involved |

---

## Setup, start to finish

### 1. Install the `ctv-pi` tool

```bash
cd receiver-pi
pip install -e .            # provides the `ctv-pi` command
```

### 2. Pair it with your backend

A Pi has no camera, so it doesn't scan the QR. In your console choose **Add a device → Raspberry
Pi**; it gives you a ready-to-paste command (the same credentials the QR would encode). SSH into the
Pi and run it, then register:

```bash
ctv-pi pair '{"endpoint":"https://your-backend/ctv","device_id":"...","device_secret":"..."}' \
  --msisdn "+8801700000001"     # the SIM's own phone number
ctv-pi register                  # announces the device to your backend
```

### 3. Receive SMS (gammu)

```bash
sudo apt install gammu gammu-smsd
sudo cp contrib/gammu-smsd.conf.example /etc/gammu-smsd.conf    # edit Device/Connection for your modem
sudo systemctl enable --now gammu-smsd
```

The example config sets `RunOnReceive = ctv-pi on-sms`, so every inbound SMS is reported to your
backend. **At this point SMS verification works** — skip to *Test it* to confirm.

### 4. Keep it online (systemd)

```bash
sudo cp contrib/ctv-pi.service /etc/systemd/system/
sudo systemctl enable --now ctv-pi        # heartbeats + drains the durable retry buffer
```

### 5. (Optional) Missed call + DTMF (Asterisk)

Only needed for voice channels. Install Asterisk with a GSM channel driver (e.g. `chan_dongle`),
add the dialplan from [`contrib/asterisk-extensions.conf.example`](contrib/asterisk-extensions.conf.example),
and copy the AGI scripts [`agi/ctv_oncall.py`](agi/ctv_oncall.py) (missed call) and
[`agi/ctv_dtmf.py`](agi/ctv_dtmf.py) (DTMF) to the paths it references. Both read the caller id from
the AGI environment, not a shell, so a spoofed caller id cannot inject commands.

**DTMF voice prompt (optional).** On a DTMF call the AGI plays a prompt, then collects the digits —
by default just a `beep`. To voice-guide the caller ("please enter the code shown on your screen,
then press the pound key"):

```bash
sudo apt install espeak-ng sox
./contrib/make-dtmf-prompt.sh                     # writes ctv-enter-pin.wav (8 kHz mono)
sudo cp ctv-enter-pin.wav /usr/share/asterisk/sounds/en/
export CTV_DTMF_PROMPT=ctv-enter-pin              # or set "dtmf_prompt" in config.json
```

Any Asterisk sound name works as `CTV_DTMF_PROMPT`; record your own if you prefer.

## Test it

From your console, **Run a test** (channel `sms`), then text the shown code from any phone to the
Pi's SIM number. The console should flip to **verified**. (For missed call / DTMF, test those
channels once Asterisk is set up in step 5.)

---

## Configuration

Read from `~/.config/calltoverify/config.json` (written by `ctv-pi pair`), overridable by environment:
`CTV_ENDPOINT`, `CTV_DEVICE_ID`, `CTV_DEVICE_SECRET`, `CTV_MSISDN`, `CTV_QUEUE`, `CTV_DTMF_PROMPT`.

## Develop

```bash
python -m unittest discover -s tests -t .
```

Tests cover the signed client (matching the Go Coordinator's HMAC scheme) and the retry buffer.
