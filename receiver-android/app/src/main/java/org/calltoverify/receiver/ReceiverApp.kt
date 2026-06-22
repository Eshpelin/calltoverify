package org.calltoverify.receiver

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import android.os.Build

/**
 * Process-wide singletons live here.
 *
 * [repository] is created once and shared by the UI, the broadcast receivers,
 * and the foreground service. The foreground-service notification channel is
 * also registered up front so the service can start immediately when paired.
 */
class ReceiverApp : Application() {

    companion object {
        /** Notification channel id for the persistent heartbeat-service notification. */
        const val CHANNEL_SERVICE = "ctv_service"

        /** Set in onCreate; safe to read from any app component. */
        lateinit var repository: ReceiverRepository
            private set
    }

    override fun onCreate() {
        super.onCreate()
        repository = ReceiverRepository.create(this)
        createNotificationChannel()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_SERVICE,
                getString(R.string.channel_service_name),
                // LOW: keep the ongoing notification quiet (no sound/vibration).
                NotificationManager.IMPORTANCE_LOW,
            ).apply {
                description = getString(R.string.channel_service_desc)
                setShowBadge(false)
            }
            getSystemService(NotificationManager::class.java)
                .createNotificationChannel(channel)
        }
    }
}
