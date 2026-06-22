package org.calltoverify.receiver.service

import android.app.Notification
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import org.calltoverify.receiver.InboundDispatcher
import org.calltoverify.receiver.R
import org.calltoverify.receiver.ReceiverApp
import org.calltoverify.receiver.ReceiverRepository
import org.calltoverify.receiver.net.CtvClient
import org.calltoverify.receiver.ui.MainActivity

/**
 * Persistent foreground service that keeps the device "online".
 *
 * On start it registers the device, then loops forever: every [HEARTBEAT_INTERVAL_MS]
 * it sends a heartbeat (refreshing the cached provisioned numbers and the online
 * flag) and drains any inbound events that were buffered while offline.
 *
 * It is started by [MainActivity] once the device is paired and re-started on boot
 * by [org.calltoverify.receiver.receiver.BootReceiver]. The ongoing notification
 * (channel [ReceiverApp.CHANNEL_SERVICE], registered in [ReceiverApp]) satisfies
 * the foreground-service requirement and lets the user see the receiver is live.
 */
class ReceiverService : Service() {

    companion object {
        private const val TAG = "ReceiverService"

        /** Stable id for the ongoing foreground notification. */
        private const val NOTIFICATION_ID = 1001

        /** How often to heartbeat and flush the retry buffer. */
        private const val HEARTBEAT_INTERVAL_MS = 60_000L

        /** Starts the service in the foreground, creating it if needed. */
        fun start(context: Context) {
            val intent = Intent(context, ReceiverService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }

        /** Stops the service (used after unpairing). */
        fun stop(context: Context) {
            context.stopService(Intent(context, ReceiverService::class.java))
        }
    }

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var loop: Job? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        startInForeground(getString(R.string.service_status_starting))
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val repository = ReceiverApp.repository
        if (!repository.paired.value) {
            // Nothing to do without credentials; shut down cleanly.
            Log.w(TAG, "Started while unpaired; stopping")
            stopSelf()
            return START_NOT_STICKY
        }

        // Only spin up the work loop once; redundant start intents are no-ops.
        if (loop == null || loop?.isActive != true) {
            loop = scope.launch { runLoop(repository) }
        }

        // STICKY: if the system kills us under memory pressure, restart so the
        // heartbeat keeps the device online.
        return START_STICKY
    }

    private suspend fun runLoop(repository: ReceiverRepository) {
        // Register once up front so the device is marked online and we learn our
        // provisioned numbers, then settle into the heartbeat cadence.
        registerOnce(repository)

        while (scope.isActive) {
            delay(HEARTBEAT_INTERVAL_MS)
            heartbeatAndDrain(repository)
        }
    }

    private fun registerOnce(repository: ReceiverRepository) {
        val pairing = repository.pairing.value ?: return
        when (val outcome = CtvClient(pairing).register()) {
            is CtvClient.Outcome.Ok -> {
                repository.updateNumbers(outcome.value.numbers)
                repository.setOnline(true)
                updateNotification(getString(R.string.service_status_online))
                Log.i(TAG, "Registered; ${outcome.value.numbers.size} number(s)")
                // Anything buffered before we were online can now go out.
                InboundDispatcher.retryQueued(repository)
            }
            is CtvClient.Outcome.HttpError -> {
                repository.setOnline(false)
                updateNotification(getString(R.string.service_status_offline))
                Log.w(TAG, "Register failed: ${outcome.code} ${outcome.body}")
            }
            is CtvClient.Outcome.NetworkError -> {
                repository.setOnline(false)
                updateNotification(getString(R.string.service_status_offline))
                Log.w(TAG, "Register network error", outcome.cause)
            }
        }
    }

    private fun heartbeatAndDrain(repository: ReceiverRepository) {
        val pairing = repository.pairing.value ?: return
        when (val outcome = CtvClient(pairing).heartbeat()) {
            is CtvClient.Outcome.Ok -> {
                if (outcome.value.numbers.isNotEmpty()) {
                    repository.updateNumbers(outcome.value.numbers)
                }
                repository.setOnline(true)
                updateNotification(getString(R.string.service_status_online))
            }
            is CtvClient.Outcome.HttpError -> {
                repository.setOnline(false)
                updateNotification(getString(R.string.service_status_offline))
                Log.w(TAG, "Heartbeat failed: ${outcome.code}")
            }
            is CtvClient.Outcome.NetworkError -> {
                repository.setOnline(false)
                updateNotification(getString(R.string.service_status_offline))
                Log.w(TAG, "Heartbeat network error", outcome.cause)
            }
        }
        // Whether or not the heartbeat succeeded, try to push out buffered events;
        // the dispatcher stops early if the link is still down.
        InboundDispatcher.retryQueued(repository)
    }

    private fun startInForeground(statusText: String) {
        val notification = buildNotification(statusText)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(
                NOTIFICATION_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC,
            )
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun updateNotification(statusText: String) {
        val manager = getSystemService(android.app.NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, buildNotification(statusText))
    }

    private fun buildNotification(statusText: String): Notification {
        val openApp = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP),
            PendingIntent.FLAG_IMMUTABLE,
        )
        return NotificationCompat.Builder(this, ReceiverApp.CHANNEL_SERVICE)
            .setContentTitle(getString(R.string.service_notification_title))
            .setContentText(statusText)
            .setSmallIcon(R.drawable.ic_notification)
            .setOngoing(true)
            .setContentIntent(openApp)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }
}
