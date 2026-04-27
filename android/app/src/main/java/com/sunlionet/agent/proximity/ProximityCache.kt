package com.sunlionet.agent.proximity

import java.util.Arrays
import java.util.LinkedHashMap

class ProximityCache(
    private val maxItems: Int = 512,
) {
    data class Entry(
        val msgId: ByteArray,
        val expiresAtMs: Long,
        val payload: ByteArray,
        val senderId: ByteArray,
        val timestampSec: Long,
        val ttlSec: Int,
    )

    private data class Key(val bytes: ByteArray) {
        override fun equals(other: Any?): Boolean {
            return other is Key && Arrays.equals(bytes, other.bytes)
        }

        override fun hashCode(): Int = Arrays.hashCode(bytes)
    }

    private val map = object : LinkedHashMap<Key, Entry>(16, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<Key, Entry>?): Boolean {
            return size > maxItems
        }
    }

    @Synchronized
    fun has(msgId: ByteArray, nowMs: Long): Boolean {
        val e = map[Key(msgId)]
        if (e == null) return false
        if (nowMs >= e.expiresAtMs) {
            map.remove(Key(msgId))
            return false
        }
        return true
    }

    @Synchronized
    fun put(e: Entry, nowMs: Long) {
        if (nowMs >= e.expiresAtMs) return
        map[Key(e.msgId.copyOf())] = e.copy(
            msgId = e.msgId.copyOf(),
            payload = e.payload.copyOf(),
            senderId = e.senderId.copyOf(),
        )
    }

    @Synchronized
    fun list(nowMs: Long, limit: Int = maxItems): List<Entry> {
        val out = ArrayList<Entry>()
        val it = map.values.iterator()
        while (it.hasNext() && out.size < limit) {
            val e = it.next()
            if (nowMs >= e.expiresAtMs) {
                it.remove()
            } else {
                out.add(e)
            }
        }
        return out
    }

    @Synchronized
    fun sweep(nowMs: Long): Int {
        var removed = 0
        val it = map.values.iterator()
        while (it.hasNext()) {
            val e = it.next()
            if (nowMs >= e.expiresAtMs) {
                it.remove()
                removed++
            }
        }
        return removed
    }
}

