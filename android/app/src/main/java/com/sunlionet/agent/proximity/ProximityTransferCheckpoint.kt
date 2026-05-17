package com.sunlionet.agent.proximity

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.security.MessageDigest
import java.util.Base64

class ProximityTransferCheckpoint(
    private val context: Context,
) {
    data class State(
        val transferId: String,
        val payloadHash: String,
        val totalChunks: Int,
        val chunkSize: Int,
        val received: BooleanArray,
    ) {
        fun nextMissing(): Int = received.indexOfFirst { !it }
        fun complete(): Boolean = nextMissing() < 0
    }

    private val dir = File(context.filesDir, "ble-checkpoints")

    fun create(payload: ByteArray, chunkSize: Int): State {
        val safeChunk = chunkSize.coerceAtLeast(1)
        val total = ((payload.size + safeChunk - 1) / safeChunk).coerceAtLeast(1)
        val hash = MessageDigest.getInstance("SHA-256").digest(payload)
        val b64 = Base64.getUrlEncoder().withoutPadding().encodeToString(hash)
        return State(
            transferId = b64.take(16),
            payloadHash = b64,
            totalChunks = total,
            chunkSize = safeChunk,
            received = BooleanArray(total),
        )
    }

    @Synchronized
    fun mark(state: State, index: Int): State {
        if (index !in 0 until state.totalChunks) return state
        state.received[index] = true
        save(state)
        return state
    }

    @Synchronized
    fun save(state: State) {
        dir.mkdirs()
        val arr = JSONArray()
        state.received.forEach { arr.put(it) }
        val json = JSONObject()
            .put("transfer_id", state.transferId)
            .put("payload_hash", state.payloadHash)
            .put("total_chunks", state.totalChunks)
            .put("chunk_size", state.chunkSize)
            .put("received", arr)
        val target = File(dir, "${state.transferId}.json")
        val tmp = File(dir, "${state.transferId}.json.tmp")
        tmp.writeText(json.toString(), Charsets.UTF_8)
        tmp.renameTo(target)
    }

    @Synchronized
    fun load(transferId: String): State? {
        val target = File(dir, "$transferId.json")
        if (!target.exists()) return null
        val json = JSONObject(target.readText(Charsets.UTF_8))
        val total = json.getInt("total_chunks")
        val arr = json.getJSONArray("received")
        if (total <= 0 || arr.length() != total) return null
        val received = BooleanArray(total) { arr.getBoolean(it) }
        return State(
            transferId = json.getString("transfer_id"),
            payloadHash = json.getString("payload_hash"),
            totalChunks = total,
            chunkSize = json.getInt("chunk_size"),
            received = received,
        )
    }
}
