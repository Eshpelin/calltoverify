package org.calltoverify.receiver.net

import java.security.SecureRandom
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

/**
 * Computes the device-request signature the CallToVerify coordinator expects.
 *
 * Every device API call carries four headers:
 *
 *   X-CTV-Device-Id   the paired device id
 *   X-CTV-Timestamp   current unix time, in SECONDS, as a string
 *   X-CTV-Nonce       a unique random token per request (replay protection)
 *   X-CTV-Signature   lowercase hex of HMAC-SHA256 over the canonical message
 *
 * The signed message is exactly:
 *
 *     timestamp + "\n" + nonce + "\n" + <raw request body bytes>
 *
 * IMPORTANT — key bytes: the coordinator keys the HMAC with the UTF-8 bytes of
 * the device_secret STRING (it does `[]byte(secret)` in Go, not a hex decode).
 * We must match that exactly, so we use `deviceSecret.toByteArray()` and do NOT
 * hex-decode the secret. See coordinator/internal/auth/auth.go (DeviceSignature).
 *
 * Because the signature covers the exact body bytes, callers must build the JSON
 * string once, sign those bytes, and send those same bytes on the wire. The
 * [CtvClient] does this; the signer just turns (secret, ts, nonce, body) into a
 * hex signature.
 */
object HmacSigner {

    private const val HMAC_ALGO = "HmacSHA256"
    private const val LF = '\n'.code.toByte()

    private val secureRandom = SecureRandom()

    /**
     * Returns the lowercase-hex HMAC-SHA256 signature for one device request.
     *
     * @param deviceSecret the paired secret; its UTF-8 bytes are the HMAC key.
     * @param timestamp    unix seconds as a string (the value sent in the header).
     * @param nonce        the per-request nonce (the value sent in the header).
     * @param body         the exact request body bytes being sent.
     */
    fun sign(deviceSecret: String, timestamp: String, nonce: String, body: ByteArray): String {
        val mac = Mac.getInstance(HMAC_ALGO)
        mac.init(SecretKeySpec(deviceSecret.toByteArray(Charsets.UTF_8), HMAC_ALGO))

        // Feed the canonical message piece by piece to avoid an intermediate copy:
        // timestamp \n nonce \n body
        mac.update(timestamp.toByteArray(Charsets.UTF_8))
        mac.update(LF)
        mac.update(nonce.toByteArray(Charsets.UTF_8))
        mac.update(LF)
        mac.update(body)

        return toHex(mac.doFinal())
    }

    /** Generates a fresh, unique nonce (128 bits of randomness, hex-encoded). */
    fun newNonce(): String {
        val bytes = ByteArray(16)
        secureRandom.nextBytes(bytes)
        return toHex(bytes)
    }

    /** Current unix time in SECONDS, as the string the header carries. */
    fun nowTimestamp(): String = (System.currentTimeMillis() / 1000L).toString()

    private fun toHex(bytes: ByteArray): String {
        val hexChars = "0123456789abcdef"
        val out = StringBuilder(bytes.size * 2)
        for (b in bytes) {
            val v = b.toInt() and 0xFF
            out.append(hexChars[v ushr 4])
            out.append(hexChars[v and 0x0F])
        }
        return out.toString()
    }
}
