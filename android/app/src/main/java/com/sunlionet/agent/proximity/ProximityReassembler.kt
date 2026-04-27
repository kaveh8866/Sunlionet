package com.sunlionet.agent.proximity

import java.util.Arrays
import java.util.concurrent.ConcurrentHashMap

class ProximityReassembler(
    private val staleAfterMs: Long = 30_000L,
) {
    private data class Key(val bytes: ByteArray) {
        override fun equals(other: Any?): Boolean {
            return other is Key && Arrays.equals(bytes, other.bytes)
        }

        override fun hashCode(): Int = Arrays.hashCode(bytes)
    }

    private data class Partial(
        val msgId: ByteArray,
        val senderId: ByteArray,
        val timestampSec: Long,
        val ttlSec: Int,
        var hop: Int,
        val chunkCount: Int,
        val chunks: Array<ByteArray?>,
        var received: Int,
        var lastAtMs: Long,
    )

    private val parts = ConcurrentHashMap<Key, Partial>()

    fun add(frame: ProximityProtocol.ChunkFrame, nowMs: Long): ProximityProtocol.Message? {
        val key = Key(frame.msgId)
        val p = parts.compute(key) { _, existing ->
            if (existing == null) {
                Partial(
                    msgId = frame.msgId.copyOf(),
                    senderId = frame.senderId.copyOf(),
                    timestampSec = frame.timestampSec,
                    ttlSec = frame.ttlSec,
                    hop = frame.hop,
                    chunkCount = frame.chunkCount,
                    chunks = arrayOfNulls(frame.chunkCount),
                    received = 0,
                    lastAtMs = nowMs,
                )
            } else {
                existing
            }
        } ?: return null

        if (p.chunkCount != frame.chunkCount) return null
        if (p.timestampSec != frame.timestampSec || p.ttlSec != frame.ttlSec) return null
        if (p.chunks[frame.chunkIndex] == null) {
            p.chunks[frame.chunkIndex] = frame.data.copyOf()
            p.received += 1
        }
        p.lastAtMs = nowMs
        p.hop = frame.hop

        if (p.received < p.chunkCount) return null

        var total = 0
        for (i in 0 until p.chunkCount) {
            total += p.chunks[i]?.size ?: 0
        }
        val payload = ByteArray(total)
        var off = 0
        for (i in 0 until p.chunkCount) {
            val c = p.chunks[i] ?: ByteArray(0)
            System.arraycopy(c, 0, payload, off, c.size)
            off += c.size
        }

        val computed = ProximityProtocol.computeMsgId(p.senderId, p.timestampSec, p.ttlSec, payload)
        if (!Arrays.equals(computed, p.msgId)) {
            parts.remove(key)
            return null
        }
        parts.remove(key)
        return ProximityProtocol.Message(
            msgId = p.msgId,
            senderId = p.senderId,
            timestampSec = p.timestampSec,
            ttlSec = p.ttlSec,
            hop = p.hop,
            payload = payload,
        )
    }

    fun sweep(nowMs: Long): Int {
        var removed = 0
        for ((k, p) in parts.entries) {
            if (nowMs - p.lastAtMs >= staleAfterMs) {
                parts.remove(k)
                removed++
            }
        }
        return removed
    }
}

