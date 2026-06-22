package org.calltoverify.receiver.data

import org.json.JSONException
import org.json.JSONObject

/** Decodes a scanned QR payload into a [Pairing], or returns a clear error. */
object PairingParser {

    sealed interface Result {
        data class Ok(val pairing: Pairing) : Result
        data class Error(val message: String) : Result
    }

    /**
     * Parses the pairing JSON:
     *
     *   {"endpoint":"https://...","device_id":"<uuid>","device_secret":"<hex>"}
     *
     * Validates that all three fields are present and that the endpoint is an
     * http(s) URL, so we fail fast on a wrong/garbage QR instead of registering
     * with nonsense.
     */
    fun parse(raw: String): Result {
        val obj = try {
            JSONObject(raw.trim())
        } catch (e: JSONException) {
            return Result.Error("Not a CallToVerify pairing QR (invalid JSON).")
        }

        val endpoint = obj.optString("endpoint").trim()
        val deviceId = obj.optString("device_id").trim()
        val deviceSecret = obj.optString("device_secret").trim()

        if (endpoint.isEmpty() || deviceId.isEmpty() || deviceSecret.isEmpty()) {
            return Result.Error("Pairing QR is missing endpoint, device_id, or device_secret.")
        }
        if (!endpoint.startsWith("http://") && !endpoint.startsWith("https://")) {
            return Result.Error("Endpoint must be an http(s) URL.")
        }

        return Result.Ok(
            Pairing(
                endpoint = endpoint.trimEnd('/'),
                deviceId = deviceId,
                deviceSecret = deviceSecret,
            ),
        )
    }
}
