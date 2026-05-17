package com.sunlionet.agent.proximity

import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.security.MessageDigest
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

object ProximityProtocol {
    private const val MAGIC0: Byte = 'S'.code.toByte()
    private const val MAGIC1: Byte = 'L'.code.toByte()
    private const val VERSION: Byte = 1
    private const val KIND_CHUNK: Byte = 1

    private const val MSG_ID_LEN = 16
    private const val SENDER_ID_LEN = 8
    private const val HEADER_LEN = 2 + 1 + 1 + MSG_ID_LEN + SENDER_ID_LEN + 4 + 2 + 1 + 2 + 2
    private const val ADV_MAGIC0: Byte = 'S'.code.toByte()
    private const val ADV_MAGIC1: Byte = 'M'.code.toByte()

    data class Message(
        val msgId: ByteArray,
        val senderId: ByteArray,
        val timestampSec: Long,
        val ttlSec: Int,
        val hop: Int,
        val payload: ByteArray,
    )

    data class ChunkFrame(
        val msgId: ByteArray,
        val senderId: ByteArray,
        val timestampSec: Long,
        val ttlSec: Int,
        val hop: Int,
        val chunkIndex: Int,
        val chunkCount: Int,
        val data: ByteArray,
    )

    data class AdvertSignal(
        val epoch: Long,
        val ephemeralNodeId: ByteArray,
        val configVersionHash: ByteArray,
    )

    fun computeMsgId(senderId: ByteArray, timestampSec: Long, ttlSec: Int, payload: ByteArray): ByteArray {
        val digest = MessageDigest.getInstance("SHA-256")
        digest.update(senderId)
        val bb = ByteBuffer.allocate(8 + 4).order(ByteOrder.LITTLE_ENDIAN)
        bb.putLong(timestampSec)
        bb.putInt(ttlSec)
        digest.update(bb.array())
        digest.update(payload)
        val full = digest.digest()
        return full.copyOfRange(0, MSG_ID_LEN)
    }

    fun newMessage(senderId: ByteArray, nowMs: Long, ttlSec: Int, payload: ByteArray): Message {
        val ts = nowMs / 1000L
        val id = computeMsgId(senderId, ts, ttlSec, payload)
        return Message(
            msgId = id,
            senderId = senderId.copyOf(),
            timestampSec = ts,
            ttlSec = ttlSec,
            hop = 0,
            payload = payload.copyOf(),
        )
    }

    fun chunk(msg: Message, maxFrameBytes: Int): List<ByteArray> {
        val maxData = (maxFrameBytes - HEADER_LEN).coerceAtLeast(1)
        val count = ((msg.payload.size + maxData - 1) / maxData).coerceAtLeast(1)
        val out = ArrayList<ByteArray>(count)
        for (i in 0 until count) {
            val start = i * maxData
            val end = (start + maxData).coerceAtMost(msg.payload.size)
            val chunkData = if (start < end) msg.payload.copyOfRange(start, end) else ByteArray(0)
            val frame = encodeChunk(
                ChunkFrame(
                    msgId = msg.msgId,
                    senderId = msg.senderId,
                    timestampSec = msg.timestampSec,
                    ttlSec = msg.ttlSec,
                    hop = msg.hop,
                    chunkIndex = i,
                    chunkCount = count,
                    data = chunkData,
                ),
            )
            out.add(frame)
        }
        return out
    }

    fun encodeChunk(f: ChunkFrame): ByteArray {
        val bb = ByteBuffer.allocate(HEADER_LEN + f.data.size).order(ByteOrder.LITTLE_ENDIAN)
        bb.put(MAGIC0)
        bb.put(MAGIC1)
        bb.put(VERSION)
        bb.put(KIND_CHUNK)
        bb.put(f.msgId.copyOfRange(0, MSG_ID_LEN))
        bb.put(f.senderId.copyOfRange(0, SENDER_ID_LEN))
        bb.putInt(f.timestampSec.toInt())
        bb.putShort(f.ttlSec.toShort())
        bb.put(f.hop.toByte())
        bb.putShort(f.chunkIndex.toShort())
        bb.putShort(f.chunkCount.toShort())
        bb.put(f.data)
        return bb.array()
    }

    fun decodeChunk(frame: ByteArray): ChunkFrame? {
        if (frame.size < HEADER_LEN) return null
        val bb = ByteBuffer.wrap(frame).order(ByteOrder.LITTLE_ENDIAN)
        if (bb.get() != MAGIC0) return null
        if (bb.get() != MAGIC1) return null
        if (bb.get() != VERSION) return null
        if (bb.get() != KIND_CHUNK) return null
        val msgId = ByteArray(MSG_ID_LEN)
        bb.get(msgId)
        val senderId = ByteArray(SENDER_ID_LEN)
        bb.get(senderId)
        val ts = bb.int.toLong() and 0xffffffffL
        val ttl = bb.short.toInt() and 0xffff
        val hop = bb.get().toInt() and 0xff
        val idx = bb.short.toInt() and 0xffff
        val cnt = bb.short.toInt() and 0xffff
        if (ttl <= 0) return null
        if (cnt <= 0 || idx >= cnt) return null
        val data = ByteArray(bb.remaining())
        bb.get(data)
        return ChunkFrame(
            msgId = msgId,
            senderId = senderId,
            timestampSec = ts,
            ttlSec = ttl,
            hop = hop,
            chunkIndex = idx,
            chunkCount = cnt,
            data = data,
        )
    }

    fun expiresAtMs(timestampSec: Long, ttlSec: Int): Long {
        return (timestampSec + ttlSec.toLong()) * 1000L
    }

    fun buildAdvertSignal(secret: ByteArray, nodeId: ByteArray, configVersion: ByteArray, nowMs: Long): ByteArray {
        require(secret.isNotEmpty())
        require(nodeId.isNotEmpty())
        require(configVersion.isNotEmpty())
        val epoch = nowMs / 90_000L
        val epochBytes = ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(epoch.toInt()).array()
        val mac = Mac.getInstance("HmacSHA256")
        mac.init(SecretKeySpec(secret, "HmacSHA256"))
        mac.update(nodeId)
        mac.update(epochBytes)
        val eph = mac.doFinal()
        val version = MessageDigest.getInstance("SHA-256").digest(configVersion)
        val out = ByteBuffer.allocate(ProximityConstants.ADV_PAYLOAD_LEN).order(ByteOrder.LITTLE_ENDIAN)
        out.put(ADV_MAGIC0)
        out.put(ADV_MAGIC1)
        out.put(1)
        out.put(0)
        out.putInt(epoch.toInt())
        out.put(eph.copyOfRange(0, 8))
        out.put(version.copyOfRange(0, 6))
        return out.array()
    }

    fun parseAdvertSignal(raw: ByteArray): AdvertSignal? {
        if (raw.size != ProximityConstants.ADV_PAYLOAD_LEN) return null
        val bb = ByteBuffer.wrap(raw).order(ByteOrder.LITTLE_ENDIAN)
        if (bb.get() != ADV_MAGIC0) return null
        if (bb.get() != ADV_MAGIC1) return null
        if (bb.get() != 1.toByte()) return null
        bb.get()
        val epoch = bb.int.toLong() and 0xffffffffL
        val node = ByteArray(8)
        bb.get(node)
        val hash = ByteArray(6)
        bb.get(hash)
        return AdvertSignal(epoch, node, hash)
    }
}
