package org.calltoverify.receiver.net

import android.util.Log
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.calltoverify.receiver.data.InboundEvent
import org.calltoverify.receiver.data.InboundResult
import org.calltoverify.receiver.data.Pairing
import org.calltoverify.receiver.data.ProvisionedNumber
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import java.io.IOException
import java.util.concurrent.TimeUnit

/**
 * Talks to the developer's backend (the embedded CallToVerify engine) over the
 * device API. Every call is HMAC-signed via [HmacSigner].
 *
 * Endpoints, all POST and relative to the paired [Pairing.endpoint]:
 *   - /devices/register   -> device config + provisioned numbers, marks online.
 *   - /devices/heartbeat   -> liveness; returns numbers. Body is `{}`.
 *   - /inbound             -> report an SMS/missed-call signal; coordinator matches.
 *
 * Signing detail that callers MUST respect: the signature covers the EXACT body
 * bytes sent. So we build the JSON string once, capture its bytes, sign those
 * bytes, and hand the very same bytes to OkHttp as the request body.
 *
 * All methods are blocking and must be called off the main thread.
 */
class CtvClient(private val pairing: Pairing) {

    companion object {
        private const val TAG = "CtvClient"
        private val JSON = "application/json; charset=utf-8".toMediaType()
    }

    private val http = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(20, TimeUnit.SECONDS)
        .writeTimeout(20, TimeUnit.SECONDS)
        .retryOnConnectionFailure(true)
        .build()

    /** Outcome of a device call: either a parsed value, or a typed failure. */
    sealed interface Outcome<out T> {
        data class Ok<T>(val value: T) : Outcome<T>

        /** HTTP completed but the server rejected/erred. [retryable] true for 5xx. */
        data class HttpError(val code: Int, val body: String, val retryable: Boolean) :
            Outcome<Nothing>

        /** Network failure (offline, timeout, DNS). Always worth retrying. */
        data class NetworkError(val cause: Throwable) : Outcome<Nothing>
    }

    /** Data returned by register/heartbeat. */
    data class DeviceState(
        val deviceId: String,
        val type: String?,
        val capabilities: List<String>,
        val numbers: List<ProvisionedNumber>,
    )

    /** POST /devices/register. Marks the device online and returns its config. */
    fun register(): Outcome<DeviceState> {
        // Register has no documented request fields; an empty JSON object keeps
        // the signed-body contract simple and forward-compatible.
        return post("/devices/register", "{}") { json ->
            parseDeviceState(json)
        }
    }

    /** POST /devices/heartbeat with body `{}`. Keeps the device "online". */
    fun heartbeat(): Outcome<DeviceState> {
        return post("/devices/heartbeat", "{}") { json ->
            // Heartbeat returns {"ok":true,"numbers":[...]} with no device_id;
            // reuse the same parser, defaulting device_id to the paired id.
            parseDeviceState(json)
        }
    }

    /**
     * POST /inbound for one event. The `number` field is the receiver's own
     * provisioned MSISDN (already resolved by the caller), `sender` is the user's
     * number, and `body` is the SMS text or "" for a missed call.
     */
    fun inbound(event: InboundEvent): Outcome<InboundResult> {
        val payload = JSONObject()
            .put("number", event.number)
            .put("type", event.type.wire)
            .put("sender", event.sender)
            .put("body", event.body)
            .toString()
        return post("/inbound", payload) { json ->
            InboundResult(
                matched = json.optBoolean("matched", false),
                sessionId = json.optString("session_id").ifEmpty { null },
                reason = json.optString("reason").ifEmpty { null },
            )
        }
    }

    // --- Internal signed POST ---

    private fun <T> post(path: String, jsonBody: String, parse: (JSONObject) -> T): Outcome<T> {
        // Build the exact bytes once, then sign exactly those bytes.
        val bodyBytes = jsonBody.toByteArray(Charsets.UTF_8)
        val timestamp = HmacSigner.nowTimestamp()
        val nonce = HmacSigner.newNonce()
        val signature = HmacSigner.sign(pairing.deviceSecret, timestamp, nonce, bodyBytes)

        val url = pairing.endpoint.trimEnd('/') + path
        val request = Request.Builder()
            .url(url)
            .post(bodyBytes.toRequestBody(JSON))
            .header("Content-Type", "application/json")
            .header("X-CTV-Device-Id", pairing.deviceId)
            .header("X-CTV-Timestamp", timestamp)
            .header("X-CTV-Nonce", nonce)
            .header("X-CTV-Signature", signature)
            .build()

        return try {
            http.newCall(request).execute().use { resp ->
                val text = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    Log.w(TAG, "POST $path -> ${resp.code}: $text")
                    // 5xx (and 429) are transient; 4xx are our fault and not retried.
                    val retryable = resp.code >= 500 || resp.code == 429
                    return Outcome.HttpError(resp.code, text, retryable)
                }
                val json = try {
                    if (text.isBlank()) JSONObject() else JSONObject(text)
                } catch (e: JSONException) {
                    Log.w(TAG, "POST $path returned non-JSON body", e)
                    return Outcome.HttpError(resp.code, text, retryable = false)
                }
                Outcome.Ok(parse(json))
            }
        } catch (e: IOException) {
            Log.w(TAG, "POST $path network error", e)
            Outcome.NetworkError(e)
        }
    }

    private fun parseDeviceState(json: JSONObject): DeviceState {
        val capabilities = json.optJSONArray("capabilities").toStringList()
        val numbers = json.optJSONArray("numbers").toNumberList()
        return DeviceState(
            deviceId = json.optString("device_id").ifEmpty { pairing.deviceId },
            type = json.optString("type").ifEmpty { null },
            capabilities = capabilities,
            numbers = numbers,
        )
    }

    private fun JSONArray?.toStringList(): List<String> {
        if (this == null) return emptyList()
        return buildList {
            for (i in 0 until length()) add(getString(i))
        }
    }

    private fun JSONArray?.toNumberList(): List<ProvisionedNumber> {
        if (this == null) return emptyList()
        return buildList {
            for (i in 0 until length()) {
                val o = optJSONObject(i) ?: continue
                add(
                    ProvisionedNumber(
                        msisdn = o.optString("msisdn"),
                        channels = o.optJSONArray("channels").toStringList(),
                        status = o.optString("status"),
                    ),
                )
            }
        }
    }
}
