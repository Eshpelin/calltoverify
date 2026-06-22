package org.calltoverify.receiver.data

import android.content.Context
import android.util.Log
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject

/**
 * A tiny durable FIFO buffer for inbound events that could not be delivered.
 *
 * When an /inbound POST fails (offline, server down, transient 5xx), the event
 * is appended here and retried later by the [ReceiverService]. Persistence is a
 * single JSON array in a private SharedPreferences file, which is plenty for the
 * low volume a single receiver phone sees, and survives process death/reboot.
 *
 * Access is synchronized so the SMS receiver, call receiver, and service retry
 * loop can enqueue/drain concurrently without corrupting the file.
 */
class InboundQueue private constructor(private val context: Context) {

    companion object {
        private const val TAG = "InboundQueue"
        private const val FILE = "ctv_inbound_queue"
        private const val KEY = "events"

        /** Cap the buffer so a long outage can't grow it without bound. */
        private const val MAX_ENTRIES = 500

        fun create(context: Context): InboundQueue = InboundQueue(context.applicationContext)
    }

    private val lock = Any()
    private val prefs = context.getSharedPreferences(FILE, Context.MODE_PRIVATE)

    /** Appends an event to the tail of the queue (dropping the oldest if full). */
    fun enqueue(event: InboundEvent) {
        synchronized(lock) {
            val list = readAll().toMutableList()
            list.add(event)
            while (list.size > MAX_ENTRIES) list.removeAt(0)
            writeAll(list)
        }
    }

    /** Returns a snapshot of all queued events without removing them. */
    fun peekAll(): List<InboundEvent> = synchronized(lock) { readAll() }

    /** Number of buffered events awaiting delivery. */
    fun size(): Int = synchronized(lock) { readAll().size }

    /**
     * Removes the first event that matches [event] by value (used after a
     * successful retry). Matching by value rather than index avoids races if the
     * queue changed between peek and remove.
     */
    fun remove(event: InboundEvent) {
        synchronized(lock) {
            val list = readAll().toMutableList()
            val idx = list.indexOfFirst {
                it.number == event.number &&
                    it.type == event.type &&
                    it.sender == event.sender &&
                    it.body == event.body &&
                    it.receivedAtMillis == event.receivedAtMillis
            }
            if (idx >= 0) {
                list.removeAt(idx)
                writeAll(list)
            }
        }
    }

    // --- JSON persistence ---

    private fun readAll(): List<InboundEvent> {
        val raw = prefs.getString(KEY, null) ?: return emptyList()
        return try {
            val arr = JSONArray(raw)
            buildList {
                for (i in 0 until arr.length()) {
                    val o = arr.getJSONObject(i)
                    val type = if (o.optString("type") == InboundType.CALL.wire) {
                        InboundType.CALL
                    } else {
                        InboundType.SMS
                    }
                    add(
                        InboundEvent(
                            number = o.optString("number"),
                            type = type,
                            sender = o.optString("sender"),
                            body = o.optString("body"),
                            receivedAtMillis = o.optLong("received_at", System.currentTimeMillis()),
                        ),
                    )
                }
            }
        } catch (e: JSONException) {
            Log.w(TAG, "Corrupt queue file; clearing", e)
            prefs.edit().remove(KEY).apply()
            emptyList()
        }
    }

    private fun writeAll(events: List<InboundEvent>) {
        val arr = JSONArray()
        for (e in events) {
            arr.put(
                JSONObject()
                    .put("number", e.number)
                    .put("type", e.type.wire)
                    .put("sender", e.sender)
                    .put("body", e.body)
                    .put("received_at", e.receivedAtMillis),
            )
        }
        prefs.edit().putString(KEY, arr.toString()).apply()
    }
}
