package org.calltoverify.receiver.data

import android.content.Context
import android.content.SharedPreferences
import android.util.Log
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject

/**
 * Persists the pairing credentials and the last-known provisioned numbers.
 *
 * The device_secret is sensitive (it signs every request), so this is backed by
 * [EncryptedSharedPreferences] using a hardware-backed master key where the
 * device supports it. If the encrypted store can't be created (rare, e.g. a
 * corrupt keystore), we fall back to plain SharedPreferences and log loudly so
 * the failure is visible rather than silent.
 */
class CredentialStore private constructor(private val prefs: SharedPreferences) {

    companion object {
        private const val TAG = "CredentialStore"
        private const val FILE = "ctv_secure_prefs"

        private const val KEY_ENDPOINT = "endpoint"
        private const val KEY_DEVICE_ID = "device_id"
        private const val KEY_DEVICE_SECRET = "device_secret"
        private const val KEY_NUMBERS_JSON = "numbers_json"

        fun create(context: Context): CredentialStore {
            val prefs = try {
                val masterKey = MasterKey.Builder(context)
                    .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
                    .build()
                EncryptedSharedPreferences.create(
                    context,
                    FILE,
                    masterKey,
                    EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                    EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
                )
            } catch (t: Throwable) {
                // Keystore problems are unrecoverable for the encrypted file; rather
                // than crash on launch, degrade to plaintext and surface it in logs.
                Log.e(TAG, "EncryptedSharedPreferences unavailable; falling back to plaintext", t)
                context.getSharedPreferences("${FILE}_plain", Context.MODE_PRIVATE)
            }
            return CredentialStore(prefs)
        }
    }

    /** True once a pairing has been stored. */
    fun isPaired(): Boolean =
        !prefs.getString(KEY_ENDPOINT, null).isNullOrBlank() &&
            !prefs.getString(KEY_DEVICE_ID, null).isNullOrBlank() &&
            !prefs.getString(KEY_DEVICE_SECRET, null).isNullOrBlank()

    /** Returns the stored pairing, or null if the device hasn't been paired. */
    fun loadPairing(): Pairing? {
        val endpoint = prefs.getString(KEY_ENDPOINT, null) ?: return null
        val deviceId = prefs.getString(KEY_DEVICE_ID, null) ?: return null
        val secret = prefs.getString(KEY_DEVICE_SECRET, null) ?: return null
        if (endpoint.isBlank() || deviceId.isBlank() || secret.isBlank()) return null
        return Pairing(endpoint = endpoint, deviceId = deviceId, deviceSecret = secret)
    }

    /** Saves a pairing scanned from a QR. The endpoint is normalized (no trailing slash). */
    fun savePairing(pairing: Pairing) {
        prefs.edit()
            .putString(KEY_ENDPOINT, pairing.endpoint.trimEnd('/'))
            .putString(KEY_DEVICE_ID, pairing.deviceId.trim())
            .putString(KEY_DEVICE_SECRET, pairing.deviceSecret.trim())
            .apply()
    }

    /** Clears all stored credentials and numbers (used by "Unpair"). */
    fun clear() {
        prefs.edit().clear().apply()
    }

    /** Caches the most recent provisioned numbers so the SMS/call receivers can
     *  resolve "our own MSISDN" without a network round-trip. */
    fun saveNumbers(numbers: List<ProvisionedNumber>) {
        val arr = JSONArray()
        for (n in numbers) {
            val channels = JSONArray()
            n.channels.forEach { channels.put(it) }
            arr.put(
                JSONObject()
                    .put("msisdn", n.msisdn)
                    .put("channels", channels)
                    .put("status", n.status),
            )
        }
        prefs.edit().putString(KEY_NUMBERS_JSON, arr.toString()).apply()
    }

    /** Returns the cached provisioned numbers (possibly empty). */
    fun loadNumbers(): List<ProvisionedNumber> {
        val raw = prefs.getString(KEY_NUMBERS_JSON, null) ?: return emptyList()
        return try {
            val arr = JSONArray(raw)
            buildList {
                for (i in 0 until arr.length()) {
                    val o = arr.getJSONObject(i)
                    val channelsArr = o.optJSONArray("channels") ?: JSONArray()
                    val channels = buildList {
                        for (j in 0 until channelsArr.length()) add(channelsArr.getString(j))
                    }
                    add(
                        ProvisionedNumber(
                            msisdn = o.optString("msisdn"),
                            channels = channels,
                            status = o.optString("status"),
                        ),
                    )
                }
            }
        } catch (e: JSONException) {
            Log.w(TAG, "Could not parse cached numbers", e)
            emptyList()
        }
    }

    /**
     * Best guess at the receiver's own MSISDN for an inbound on [channel].
     *
     * Single-SIM phones have exactly one provisioned number, so we prefer the
     * first active number that advertises the channel; failing that, the first
     * active number; failing that, the first number at all.
     */
    fun primaryMsisdnFor(channel: String): String? {
        val numbers = loadNumbers()
        if (numbers.isEmpty()) return null
        return numbers.firstOrNull { it.status == "active" && channel in it.channels }?.msisdn
            ?: numbers.firstOrNull { it.status == "active" }?.msisdn
            ?: numbers.first().msisdn
    }
}
