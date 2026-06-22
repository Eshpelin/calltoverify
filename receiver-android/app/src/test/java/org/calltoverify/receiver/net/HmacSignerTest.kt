package org.calltoverify.receiver.net

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Cross-language known-answer vector for the device-signing protocol.
 *
 * This pinned digest is mirrored verbatim in the Go, Python/Pi, Node, and PHP test
 * suites (see coordinator/internal/auth/auth_test.go). A round-trip test cannot
 * catch cross-language drift because both sides move together; a fixed digest can.
 * If this test fails, the Kotlin signer no longer matches the Coordinator's
 * verifier and device authentication would silently break. Do not change the
 * expected value without updating every mirrored test.
 */
class HmacSignerTest {
    @Test
    fun deviceSignatureKnownAnswerVector() {
        val sig = HmacSigner.sign(
            deviceSecret = "s3cr3t",
            timestamp = "1700000000",
            nonce = "nonce1",
            body = "{\"a\":1}".toByteArray(Charsets.UTF_8),
        )
        assertEquals("93cffdba929d8f1c542790a0b59ca1fd239a0a2a1f909f18f25ee401e484fc24", sig)
    }
}
