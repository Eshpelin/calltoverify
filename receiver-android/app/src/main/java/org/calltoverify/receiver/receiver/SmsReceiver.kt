package org.calltoverify.receiver.receiver

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.provider.Telephony
import android.util.Log
import org.calltoverify.receiver.ReceiverApp
import org.calltoverify.receiver.data.InboundType
import org.calltoverify.receiver.InboundDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

/**
 * Receives inbound SMS and reports them to the developer's backend.
 *
 * Registered for SMS_RECEIVED_ACTION in the manifest. A single SMS can arrive as
 * several PDUs (multipart); we group parts by originating address and concatenate
 * their bodies so the developer sees the full text and a single sender.
 *
 * The actual network call is offloaded to a coroutine via [goAsync] so the
 * broadcast can return quickly while delivery (or buffering) completes.
 */
class SmsReceiver : BroadcastReceiver() {

    companion object {
        private const val TAG = "SmsReceiver"
    }

    private val scope = CoroutineScope(Dispatchers.IO)

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Telephony.Sms.Intents.SMS_RECEIVED_ACTION) return

        val messages = Telephony.Sms.Intents.getMessagesFromIntent(intent) ?: return
        if (messages.isEmpty()) return

        // Concatenate multipart parts per sender. Most inbound here is a single
        // short OTP-style SMS, but handle long messages correctly regardless.
        val bySender = LinkedHashMap<String, StringBuilder>()
        for (msg in messages) {
            val sender = msg.originatingAddress ?: continue
            val body = msg.messageBody ?: ""
            bySender.getOrPut(sender) { StringBuilder() }.append(body)
        }
        if (bySender.isEmpty()) return

        val repository = ReceiverApp.repository
        if (!repository.paired.value) {
            Log.w(TAG, "SMS received but device not paired; ignoring")
            return
        }

        // Keep the broadcast alive while we post off the main thread.
        val pending = goAsync()
        scope.launch {
            try {
                for ((sender, builder) in bySender) {
                    val text = builder.toString()
                    Log.i(TAG, "Inbound SMS from $sender (${text.length} chars)")
                    InboundDispatcher.handle(
                        repository = repository,
                        channel = "sms",
                        type = InboundType.SMS,
                        sender = sender,
                        body = text,
                    )
                }
            } catch (t: Throwable) {
                Log.e(TAG, "Failed handling inbound SMS", t)
            } finally {
                pending.finish()
            }
        }
    }
}
