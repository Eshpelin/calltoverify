package org.calltoverify.receiver

import android.util.Log
import org.calltoverify.receiver.data.InboundEvent
import org.calltoverify.receiver.data.InboundType
import org.calltoverify.receiver.net.CtvClient

/**
 * Turns a raw inbound signal into a signed /inbound POST, with offline buffering.
 *
 * Both [org.calltoverify.receiver.receiver.SmsReceiver] and the call listener
 * funnel through here so the "resolve our MSISDN, sign, post, or buffer on
 * failure" logic lives in exactly one place. The foreground service uses
 * [retryQueued] to drain anything that was buffered.
 *
 * All methods block and must run off the main thread.
 */
object InboundDispatcher {

    private const val TAG = "InboundDispatcher"

    /**
     * Builds an [InboundEvent] for an inbound on [channel], resolving the
     * receiver's own MSISDN from the cached provisioned numbers, then attempts
     * delivery (buffering on failure).
     *
     * @param channel "sms" or "call" (used to pick the right provisioned number).
     * @param type    the inbound type to report.
     * @param sender  the user's number (SMS originator or caller id).
     * @param body    the SMS text, or "" for a missed call.
     * @return true if the event was delivered now, false if it was buffered.
     */
    fun handle(
        repository: ReceiverRepository,
        channel: String,
        type: InboundType,
        sender: String,
        body: String,
    ): Boolean {
        val pairing = repository.pairing.value
        if (pairing == null) {
            Log.w(TAG, "Inbound ignored: device not paired")
            return false
        }

        val number = repository.credentials.primaryMsisdnFor(channel)
        if (number.isNullOrBlank()) {
            // We don't yet know our own provisioned MSISDN (never registered, or
            // server returned none). Buffer with a placeholder so the event isn't
            // lost; the retry loop will re-resolve the number before sending.
            Log.w(TAG, "No provisioned MSISDN yet for channel=$channel; buffering")
            val event = InboundEvent(number = "", type = type, sender = sender, body = body)
            repository.queue.enqueue(event)
            repository.refreshPendingCount()
            repository.logInbound(event, "buffered (no number yet)")
            return false
        }

        val event = InboundEvent(number = number, type = type, sender = sender, body = body)
        return deliverOrBuffer(repository, CtvClient(pairing), event)
    }

    /**
     * Drains the offline buffer. For each queued event, re-resolves a missing
     * MSISDN if needed, then tries to deliver it. Stops early on a network error
     * (no point hammering an offline link) but keeps going past per-event 4xx
     * rejections, which it drops as permanently unsendable.
     */
    fun retryQueued(repository: ReceiverRepository) {
        val pairing = repository.pairing.value ?: return
        val client = CtvClient(pairing)
        val pending = repository.queue.peekAll()
        if (pending.isEmpty()) return

        Log.i(TAG, "Retrying ${pending.size} buffered event(s)")
        for (queued in pending) {
            // Fill in a number that was unknown when the event was first buffered.
            val event = if (queued.number.isBlank()) {
                val resolved = repository.credentials.primaryMsisdnFor(queued.type.wire)
                if (resolved.isNullOrBlank()) {
                    Log.w(TAG, "Still no MSISDN; leaving event buffered")
                    continue
                }
                queued.copy(number = resolved)
            } else {
                queued
            }

            when (val outcome = client.inbound(event)) {
                is CtvClient.Outcome.Ok -> {
                    repository.queue.remove(queued)
                    repository.logInbound(
                        event,
                        if (outcome.value.matched) "matched (retry)" else "no match (retry)",
                    )
                }
                is CtvClient.Outcome.HttpError -> {
                    if (outcome.retryable) {
                        Log.w(TAG, "Retryable ${outcome.code}; stopping drain")
                        break
                    }
                    // 4xx: the server will never accept this; drop it.
                    Log.w(TAG, "Dropping unsendable event: ${outcome.code} ${outcome.body}")
                    repository.queue.remove(queued)
                    repository.logInbound(event, "dropped (${outcome.code})")
                }
                is CtvClient.Outcome.NetworkError -> {
                    Log.w(TAG, "Offline; stopping drain")
                    break
                }
            }
        }
        repository.refreshPendingCount()
    }

    private fun deliverOrBuffer(
        repository: ReceiverRepository,
        client: CtvClient,
        event: InboundEvent,
    ): Boolean {
        return when (val outcome = client.inbound(event)) {
            is CtvClient.Outcome.Ok -> {
                val matched = outcome.value.matched
                repository.logInbound(event, if (matched) "matched" else "no match")
                true
            }
            is CtvClient.Outcome.HttpError -> {
                if (outcome.retryable) {
                    repository.queue.enqueue(event)
                    repository.refreshPendingCount()
                    repository.logInbound(event, "buffered (${outcome.code})")
                    false
                } else {
                    // Permanent rejection (e.g. bad request); log and discard.
                    repository.logInbound(event, "rejected (${outcome.code})")
                    false
                }
            }
            is CtvClient.Outcome.NetworkError -> {
                repository.queue.enqueue(event)
                repository.refreshPendingCount()
                repository.logInbound(event, "buffered (offline)")
                false
            }
        }
    }
}
