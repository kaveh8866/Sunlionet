package com.sunlionet.agent.proximity

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertEquals
import org.junit.Test
import java.util.Random

class ProximityProtocolTest {
    @Test
    fun chunk_encode_decode_reassemble() {
        val sender = ByteArray(8) { it.toByte() }
        val payload = ByteArray(900) { ((it * 31) % 251).toByte() }
        val msg = ProximityProtocol.newMessage(sender, nowMs = 1_700_000_000_000L, ttlSec = 60, payload = payload)
        val frames = ProximityProtocol.chunk(msg, maxFrameBytes = 120)
        val decoded = frames.mapNotNull { ProximityProtocol.decodeChunk(it) }
        val r = ProximityReassembler(staleAfterMs = 10_000L)
        val nowMs = 1_700_000_000_100L

        val shuffled = decoded.toMutableList()
        shuffled.shuffle(Random(7))
        var out: ProximityProtocol.Message? = null
        shuffled.forEach { f ->
            val m = r.add(f, nowMs)
            if (m != null) out = m
        }

        val finalMsg = out
        assertNotNull(finalMsg)
        assertArrayEquals(msg.msgId, finalMsg!!.msgId)
        assertArrayEquals(msg.senderId, finalMsg.senderId)
        assertArrayEquals(msg.payload, finalMsg.payload)
    }

    @Test
    fun advert_signal_is_legacy_ble_sized_and_obfuscated() {
        val secret = "secret".toByteArray()
        val node = ByteArray(8) { it.toByte() }
        val version = "bundle-version".toByteArray()
        val adv = ProximityProtocol.buildAdvertSignal(secret, node, version, 1_700_000_000_000L)

        assertEquals(ProximityConstants.ADV_PAYLOAD_LEN, adv.size)
        val parsed = ProximityProtocol.parseAdvertSignal(adv)
        assertNotNull(parsed)
        assertEquals(8, parsed!!.ephemeralNodeId.size)
        assertEquals(6, parsed.configVersionHash.size)
        if (node.contentEquals(parsed.ephemeralNodeId)) {
            throw AssertionError("advertisement leaked stable node id")
        }
    }
}
