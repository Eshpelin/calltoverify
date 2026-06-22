package org.calltoverify.receiver

import android.content.Context
import org.calltoverify.receiver.data.CredentialStore
import org.calltoverify.receiver.data.InboundEvent
import org.calltoverify.receiver.data.InboundLogEntry
import org.calltoverify.receiver.data.InboundQueue
import org.calltoverify.receiver.data.Pairing
import org.calltoverify.receiver.data.ProvisionedNumber
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

/**
 * Single source of truth shared across the app process.
 *
 * The broadcast receivers, the foreground service, and the Compose UI all read
 * and write through this one instance (held by [ReceiverApp]). It wraps the
 * durable stores ([CredentialStore], [InboundQueue]) and exposes observable
 * [StateFlow]s for the UI plus a rolling in-memory event log.
 */
class ReceiverRepository private constructor(
    val credentials: CredentialStore,
    val queue: InboundQueue,
) {
    companion object {
        /** How many recent inbound events to keep in the on-screen log. */
        private const val LOG_CAPACITY = 50

        fun create(context: Context): ReceiverRepository =
            ReceiverRepository(
                credentials = CredentialStore.create(context),
                queue = InboundQueue.create(context),
            )
    }

    private val _paired = MutableStateFlow(credentials.isPaired())
    val paired: StateFlow<Boolean> = _paired.asStateFlow()

    private val _pairing = MutableStateFlow(credentials.loadPairing())
    val pairing: StateFlow<Pairing?> = _pairing.asStateFlow()

    private val _online = MutableStateFlow(false)
    val online: StateFlow<Boolean> = _online.asStateFlow()

    private val _numbers = MutableStateFlow(credentials.loadNumbers())
    val numbers: StateFlow<List<ProvisionedNumber>> = _numbers.asStateFlow()

    private val _pendingCount = MutableStateFlow(queue.size())
    val pendingCount: StateFlow<Int> = _pendingCount.asStateFlow()

    private val _log = MutableStateFlow<List<InboundLogEntry>>(emptyList())
    val log: StateFlow<List<InboundLogEntry>> = _log.asStateFlow()

    /** Persists a freshly scanned pairing and flips the app into the paired state. */
    fun savePairing(p: Pairing) {
        credentials.savePairing(p)
        _pairing.value = credentials.loadPairing()
        _paired.value = credentials.isPaired()
    }

    /** Forgets all credentials and resets observable state (the "Unpair" action). */
    fun unpair() {
        credentials.clear()
        _pairing.value = null
        _paired.value = false
        _online.value = false
        _numbers.value = emptyList()
    }

    /** Updates the cached provisioned numbers (after register/heartbeat). */
    fun updateNumbers(numbers: List<ProvisionedNumber>) {
        credentials.saveNumbers(numbers)
        _numbers.value = numbers
    }

    fun setOnline(online: Boolean) {
        _online.value = online
    }

    fun refreshPendingCount() {
        _pendingCount.value = queue.size()
    }

    /** Appends an entry to the rolling UI log (newest first, capped). */
    fun appendLog(entry: InboundLogEntry) {
        _log.update { current ->
            (listOf(entry) + current).take(LOG_CAPACITY)
        }
    }

    /** Convenience: log a handled inbound event with a human summary. */
    fun logInbound(event: InboundEvent, summary: String) {
        appendLog(
            InboundLogEntry(
                timestampMillis = event.receivedAtMillis,
                type = event.type,
                sender = event.sender,
                number = event.number,
                summary = summary,
            ),
        )
    }
}
