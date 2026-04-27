package com.sunlionet.agent.proximity

import java.security.KeyPair
import java.security.KeyPairGenerator
import java.security.MessageDigest
import java.security.SecureRandom
import java.security.spec.ECGenParameterSpec
import java.util.concurrent.atomic.AtomicReference

class ProximityIdentityManager(
    private val rotationMs: Long = 5 * 60_000L,
) {
    data class Identity(
        val nodeId: ByteArray,
        val pubKeyEncoded: ByteArray,
    )

    private data class State(
        val identity: Identity,
        val rotatedAtMs: Long,
    )

    private val rng = SecureRandom()
    private val state = AtomicReference<State?>(null)

    fun current(nowMs: Long = System.currentTimeMillis()): Identity {
        val s = state.get()
        if (s == null || nowMs - s.rotatedAtMs >= rotationMs) {
            val next = rotate(nowMs)
            state.set(next)
            return next.identity
        }
        return s.identity
    }

    private fun rotate(nowMs: Long): State {
        val kp = generateEcKeyPair()
        val pub = kp.public.encoded
        val bucket = (nowMs / rotationMs).toString().toByteArray(Charsets.UTF_8)
        val digest = MessageDigest.getInstance("SHA-256")
        digest.update(pub)
        digest.update(bucket)
        val full = digest.digest()
        val nodeId = full.copyOfRange(0, ProximityConstants.ADV_NODE_ID_LEN)
        return State(
            identity = Identity(nodeId = nodeId, pubKeyEncoded = pub),
            rotatedAtMs = nowMs,
        )
    }

    private fun generateEcKeyPair(): KeyPair {
        val gen = KeyPairGenerator.getInstance("EC")
        gen.initialize(ECGenParameterSpec("secp256r1"), rng)
        return gen.generateKeyPair()
    }
}

