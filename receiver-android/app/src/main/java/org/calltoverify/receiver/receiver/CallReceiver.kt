package org.calltoverify.receiver.receiver

import android.Manifest
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.telecom.TelecomManager
import android.telephony.TelephonyManager
import android.util.Log
import androidx.core.content.ContextCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import org.calltoverify.receiver.InboundDispatcher
import org.calltoverify.receiver.ReceiverApp
import org.calltoverify.receiver.data.InboundType

/**
 * Detects an incoming call, auto-rejects it, and reports it as a missed-call
 * inbound signal.
 *
 * The missed-call channel works like this: the user "calls" the provisioned
 * number, we see the ringing call and the caller id, we reject it immediately
 * (so the user is never charged — a rejected call is free), and we report the
 * caller's number to the coordinator, which matches it to a pending verification.
 *
 * Registered for ACTION_PHONE_STATE_CHANGED in the manifest. We act on the first
 * RINGING transition for a given number and de-duplicate the rapid repeat
 * broadcasts Android emits for the same call.
 *
 * Auto-reject uses [TelecomManager.endCall], which requires
 * [Manifest.permission.ANSWER_PHONE_CALLS] and API 28+. On older devices, or
 * without the permission, we still report the call but cannot reject it.
 */
class CallReceiver : BroadcastReceiver() {

    companion object {
        private const val TAG = "CallReceiver"

        /**
         * Guards against handling the same ringing call twice: Android delivers
         * PHONE_STATE repeatedly while a call rings. We remember the last number
         * we reported and ignore identical follow-ups within a short window.
         */
        @Volatile
        private var lastHandledNumber: String? = null

        @Volatile
        private var lastHandledAtMillis: Long = 0L

        /** Serializes the dedupe check-and-update against concurrent broadcasts. */
        private val dedupeLock = Any()

        /** Repeat broadcasts for the same call arrive within a couple seconds. */
        private const val DEDUPE_WINDOW_MS = 5_000L
    }

    private val scope = CoroutineScope(Dispatchers.IO)

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != TelephonyManager.ACTION_PHONE_STATE_CHANGED) return

        val state = intent.getStringExtra(TelephonyManager.EXTRA_STATE)
        if (state != TelephonyManager.EXTRA_STATE_RINGING) return

        // The caller number is only present when READ_CALL_LOG is granted; without
        // it the extra is absent, so we cannot match the call and skip it.
        val number = intent.getStringExtra(TelephonyManager.EXTRA_INCOMING_NUMBER)
        if (number.isNullOrBlank()) {
            Log.w(TAG, "Ringing call with no caller id (missing READ_CALL_LOG?); ignoring")
            return
        }

        // De-duplicate the repeated RINGING broadcasts for one physical call. The
        // check-and-update must be atomic: two broadcasts can arrive nearly together
        // and otherwise both pass the window.
        val now = System.currentTimeMillis()
        synchronized(dedupeLock) {
            if (number == lastHandledNumber && now - lastHandledAtMillis < DEDUPE_WINDOW_MS) {
                return
            }
            lastHandledNumber = number
            lastHandledAtMillis = now
        }

        val repository = ReceiverApp.repository
        if (!repository.paired.value) {
            Log.w(TAG, "Incoming call but device not paired; ignoring")
            return
        }

        // Reject as fast as possible so the call stays free for the user.
        rejectCall(context)

        Log.i(TAG, "Incoming call from $number; reporting missed-call inbound")
        val pending = goAsync()
        scope.launch {
            try {
                InboundDispatcher.handle(
                    repository = repository,
                    channel = "call",
                    type = InboundType.CALL,
                    sender = number,
                    body = "",
                )
            } catch (t: Throwable) {
                Log.e(TAG, "Failed handling inbound call", t)
            } finally {
                pending.finish()
            }
        }
    }

    /**
     * Ends the currently ringing call via [TelecomManager.endCall].
     *
     * Requires API 28+ and the ANSWER_PHONE_CALLS runtime permission; we guard on
     * both and degrade to "report but don't reject" if either is unavailable.
     */
    private fun rejectCall(context: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.P) {
            Log.w(TAG, "Auto-reject needs API 28+; reporting without rejecting")
            return
        }
        if (ContextCompat.checkSelfPermission(
                context,
                Manifest.permission.ANSWER_PHONE_CALLS,
            ) != PackageManager.PERMISSION_GRANTED
        ) {
            Log.w(TAG, "ANSWER_PHONE_CALLS not granted; reporting without rejecting")
            return
        }
        try {
            val telecom = context.getSystemService(Context.TELECOM_SERVICE) as? TelecomManager
            @Suppress("DEPRECATION") // endCall() is the supported reject path here.
            val ended = telecom?.endCall() ?: false
            if (ended) {
                Log.i(TAG, "Auto-rejected incoming call")
            } else {
                Log.w(TAG, "endCall() returned false; call may not have been rejected")
            }
        } catch (t: Throwable) {
            Log.e(TAG, "Failed to auto-reject call", t)
        }
    }
}
