package org.calltoverify.receiver.data

/**
 * The credentials decoded from a pairing QR and stored on the device.
 *
 * The QR payload is JSON:
 *
 *   {
 *     "endpoint":      "https://their-backend.com/ctv",
 *     "device_id":     "<uuid>",
 *     "device_secret": "<hex secret>"
 *   }
 *
 * [endpoint] is the base URL the device API paths are appended to, e.g.
 * `<endpoint>/devices/register`.
 */
data class Pairing(
    val endpoint: String,
    val deviceId: String,
    val deviceSecret: String,
)

/**
 * A provisioned number as returned by /devices/register and /devices/heartbeat.
 *
 *   { "msisdn": "+8801...", "channels": ["sms","call"], "status": "active" }
 *
 * The receiver learns its own SIM number(s) from here — reading the SIM number
 * directly off Android is unreliable, so we trust the server's provisioned value.
 */
data class ProvisionedNumber(
    val msisdn: String,
    val channels: List<String>,
    val status: String,
)

/** Inbound channel type, matching the coordinator's accepted `type` values. */
enum class InboundType(val wire: String) {
    SMS("sms"),
    CALL("call"),
}

/**
 * One inbound signal to report to /inbound.
 *
 * @param number the receiver phone's own provisioned MSISDN (the SIM that got
 *               the signal), NOT the sender. Required by the coordinator.
 * @param sender the user's number (SMS originating address, or caller id).
 * @param body   the SMS text, or "" for a missed call.
 */
data class InboundEvent(
    val number: String,
    val type: InboundType,
    val sender: String,
    val body: String,
    val receivedAtMillis: Long = System.currentTimeMillis(),
)

/** The coordinator's reply to /inbound: `{matched, session_id?, reason?}`. */
data class InboundResult(
    val matched: Boolean,
    val sessionId: String?,
    val reason: String?,
)

/** A log line shown in the UI for a recently handled inbound event. */
data class InboundLogEntry(
    val timestampMillis: Long,
    val type: InboundType,
    val sender: String,
    val number: String,
    val summary: String,
)
