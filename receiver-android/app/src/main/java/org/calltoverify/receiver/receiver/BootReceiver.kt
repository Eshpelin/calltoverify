package org.calltoverify.receiver.receiver

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log
import org.calltoverify.receiver.ReceiverApp
import org.calltoverify.receiver.service.ReceiverService

/**
 * Re-starts the foreground [ReceiverService] after the phone reboots.
 *
 * Registered for BOOT_COMPLETED in the manifest. We only restart the service if a
 * pairing is still stored; an unpaired device has nothing to keep online.
 */
class BootReceiver : BroadcastReceiver() {

    companion object {
        private const val TAG = "BootReceiver"
    }

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return

        if (ReceiverApp.repository.paired.value) {
            Log.i(TAG, "Boot completed and device paired; starting service")
            ReceiverService.start(context)
        } else {
            Log.i(TAG, "Boot completed but device not paired; nothing to do")
        }
    }
}
