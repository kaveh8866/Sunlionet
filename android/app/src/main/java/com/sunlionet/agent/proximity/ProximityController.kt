package com.sunlionet.agent.proximity

import android.Manifest
import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothManager
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.content.ContextCompat
import com.sunlionet.agent.BuildConfig
import com.sunlionet.agent.Logs
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.util.Arrays
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong
import kotlin.math.min

class ProximityController(
    context: Context,
) {
    private val appContext = context.applicationContext
    private val scope = CoroutineScope(Dispatchers.IO)
    private var job: Job? = null

    private val btAdapter: BluetoothAdapter? =
        (appContext.getSystemService(Context.BLUETOOTH_SERVICE) as BluetoothManager).adapter

    private val identity = ProximityIdentityManager(rotationMs = 90_000L)
    private val advertiser = ProximityBleAdvertiser(appContext)

    private val cache = ProximityCache(maxItems = 1024)
    private val reasm = ProximityReassembler(staleAfterMs = 25_000L)

    private val peers = ConcurrentHashMap<String, Peer>()
    private val clients = ConcurrentHashMap<String, ProximityGattClient>()
    private val forwardGate = ConcurrentHashMap<String, AtomicLong>()

    private val server =
        ProximityGattServer(appContext) { frame ->
            handleIncoming(frame, System.currentTimeMillis())
        }

    private val scanner =
        ProximityBleScanner(appContext) { deviceAddress, nodeId, rssi ->
            onSeen(deviceAddress, nodeId, rssi, System.currentTimeMillis())
        }

    private data class Peer(
        val address: String,
        val nodeId: ByteArray,
        var lastSeenAtMs: Long,
        var rssi: Int,
    )

    fun start() {
        if (!BuildConfig.DEBUG && !BuildConfig.TESTER_MODE) return
        if (job != null) return
        if (!hasBluetoothReady()) {
            Logs.w("proximity", "bluetooth unavailable")
            return
        }
        if (!hasPermissions()) {
            Logs.w("proximity", "missing bluetooth permissions")
            return
        }
        if (!server.start()) {
            Logs.w("proximity", "gatt server start failed")
            return
        }
        job =
            scope.launch {
                while (isActive) {
                    val nowMs = System.currentTimeMillis()
                    val id = identity.current(nowMs)
                    advertiser.start(id)
                    scanCycle()
                    reasm.sweep(nowMs)
                    cache.sweep(nowMs)
                    evictPeers(nowMs)
                }
            }
        Logs.i("proximity", "started")
    }

    fun stop() {
        val j = job ?: return
        job = null
        j.cancel()
        runCatching { scanner.stop() }
        runCatching { advertiser.stop() }
        runCatching { server.stop() }
        clients.values.forEach { runCatching { it.disconnect() } }
        clients.clear()
        peers.clear()
        Logs.i("proximity", "stopped")
    }

    fun send(payload: ByteArray, ttlSec: Int = 60) {
        if (!BuildConfig.DEBUG && !BuildConfig.TESTER_MODE) return
        val nowMs = System.currentTimeMillis()
        val id = identity.current(nowMs)
        val msg = ProximityProtocol.newMessage(id.nodeId, nowMs, ttlSec, payload)
        val frames = ProximityProtocol.chunk(msg, maxFrameBytes = 180)
        val exp = ProximityProtocol.expiresAtMs(msg.timestampSec, msg.ttlSec)
        cache.put(
            ProximityCache.Entry(
                msgId = msg.msgId,
                expiresAtMs = exp,
                payload = msg.payload,
                senderId = msg.senderId,
                timestampSec = msg.timestampSec,
                ttlSec = msg.ttlSec,
            ),
            nowMs,
        )
        frames.forEach { broadcastFrame(it) }
    }

    private suspend fun scanCycle() {
        val havePeers = clients.isNotEmpty()
        val onMs = if (havePeers) 3_000L else 6_000L
        val offMs = if (havePeers) 12_000L else 8_000L
        runCatching { scanner.startLowPower() }
        delay(onMs)
        runCatching { scanner.stop() }
        delay(offMs)
    }

    private fun onSeen(address: String, nodeId: ByteArray, rssi: Int, nowMs: Long) {
        val ours = identity.current(nowMs).nodeId
        if (Arrays.equals(ours, nodeId)) return
        val p = peers.compute(address) { _, existing ->
            if (existing == null) {
                Peer(address = address, nodeId = nodeId.copyOf(), lastSeenAtMs = nowMs, rssi = rssi)
            } else {
                existing.lastSeenAtMs = nowMs
                existing.rssi = rssi
                existing
            }
        } ?: return
        val shouldConnect = lexLessThan(ours, p.nodeId)
        if (!shouldConnect) {
            clients.remove(address)?.disconnect()
            return
        }
        val client = clients[address]
        if (client != null) return
        val device = btAdapter?.getRemoteDevice(address) ?: return
        val c =
            ProximityGattClient(
                appContext,
                device,
                onFrame = { frame -> handleIncoming(frame, System.currentTimeMillis()) },
                onReady = { onPeerReady(address) },
            )
        clients[address] = c
        c.connect()
    }

    private fun onPeerReady(address: String) {
        val client = clients[address] ?: return
        val nowMs = System.currentTimeMillis()
        cache.list(nowMs, limit = 8).forEach { e ->
            val msg =
                ProximityProtocol.Message(
                    msgId = e.msgId,
                    senderId = e.senderId,
                    timestampSec = e.timestampSec,
                    ttlSec = e.ttlSec,
                    hop = 0,
                    payload = e.payload,
                )
            val frames = ProximityProtocol.chunk(msg, maxFrameBytes = 180)
            frames.forEach { client.enqueueWrite(it) }
        }
        Logs.i("proximity", "peer ready addr=$address")
    }

    private fun handleIncoming(frame: ByteArray, nowMs: Long) {
        val decoded = ProximityProtocol.decodeChunk(frame) ?: return
        if (decoded.hop > 8) return
        val exp = ProximityProtocol.expiresAtMs(decoded.timestampSec, decoded.ttlSec)
        if (nowMs >= exp) return
        val msg = reasm.add(decoded, nowMs) ?: run {
            forwardFrame(decoded, frame, nowMs)
            return
        }
        if (cache.has(msg.msgId, nowMs)) return
        val expiresAtMs = ProximityProtocol.expiresAtMs(msg.timestampSec, msg.ttlSec)
        cache.put(
            ProximityCache.Entry(
                msgId = msg.msgId,
                expiresAtMs = expiresAtMs,
                payload = msg.payload,
                senderId = msg.senderId,
                timestampSec = msg.timestampSec,
                ttlSec = msg.ttlSec,
            ),
            nowMs,
        )
        Logs.i("proximity", "msg id=${hex16(msg.msgId)} hop=${msg.hop}")
        forwardMessage(msg, nowMs)
    }

    private fun forwardFrame(decoded: ProximityProtocol.ChunkFrame, raw: ByteArray, nowMs: Long) {
        if (decoded.hop >= 8) return
        if (!shouldForward(decoded.msgId, nowMs, minEveryMs = 150L)) return
        val nextHop = decoded.hop + 1
        val patched = raw.copyOf()
        if (patched.size >= 2 + 1 + 1 + 16 + 8 + 4 + 2 + 1) {
            val hopOffset = 2 + 1 + 1 + 16 + 8 + 4 + 2
            patched[hopOffset] = nextHop.toByte()
            broadcastFrame(patched)
        }
    }

    private fun forwardMessage(msg: ProximityProtocol.Message, nowMs: Long) {
        if (msg.hop >= 8) return
        if (!shouldForward(msg.msgId, nowMs, minEveryMs = 800L)) return
        val next =
            ProximityProtocol.Message(
                msgId = msg.msgId,
                senderId = msg.senderId,
                timestampSec = msg.timestampSec,
                ttlSec = msg.ttlSec,
                hop = msg.hop + 1,
                payload = msg.payload,
            )
        val frames = ProximityProtocol.chunk(next, maxFrameBytes = 180)
        val minDelayMs = 50L
        val jitter = min(250L, minDelayMs + (nowMs % 200L))
        scope.launch {
            delay(jitter)
            frames.forEach { broadcastFrame(it) }
        }
    }

    private fun broadcastFrame(frame: ByteArray) {
        server.notifyAll(frame)
        clients.values.forEach { it.enqueueWrite(frame) }
    }

    private fun shouldForward(msgId: ByteArray, nowMs: Long, minEveryMs: Long): Boolean {
        val k = hex16(msgId)
        val last = forwardGate.computeIfAbsent(k) { AtomicLong(0L) }
        while (true) {
            val prev = last.get()
            if (nowMs - prev < minEveryMs) return false
            if (last.compareAndSet(prev, nowMs)) return true
        }
    }

    private fun evictPeers(nowMs: Long) {
        val staleMs = 60_000L
        peers.entries.removeIf { (_, p) ->
            val stale = nowMs - p.lastSeenAtMs >= staleMs
            if (stale) {
                clients.remove(p.address)?.disconnect()
            }
            stale
        }
    }

    private fun hasBluetoothReady(): Boolean {
        val a = btAdapter ?: return false
        return a.isEnabled
    }

    private fun hasPermissions(): Boolean {
        return if (Build.VERSION.SDK_INT >= 31) {
            has(Manifest.permission.BLUETOOTH_SCAN) &&
                has(Manifest.permission.BLUETOOTH_CONNECT) &&
                has(Manifest.permission.BLUETOOTH_ADVERTISE)
        } else {
            has(Manifest.permission.BLUETOOTH) &&
                has(Manifest.permission.BLUETOOTH_ADMIN) &&
                has(Manifest.permission.ACCESS_FINE_LOCATION)
        }
    }

    private fun has(p: String): Boolean {
        return ContextCompat.checkSelfPermission(appContext, p) == PackageManager.PERMISSION_GRANTED
    }

    private fun lexLessThan(a: ByteArray, b: ByteArray): Boolean {
        val n = min(a.size, b.size)
        for (i in 0 until n) {
            val ai = a[i].toInt() and 0xff
            val bi = b[i].toInt() and 0xff
            if (ai < bi) return true
            if (ai > bi) return false
        }
        return a.size < b.size
    }

    private fun hex16(b: ByteArray): String {
        val n = min(8, b.size)
        val sb = StringBuilder(n * 2)
        for (i in 0 until n) {
            val v = b[i].toInt() and 0xff
            val hi = "0123456789abcdef"[v ushr 4]
            val lo = "0123456789abcdef"[v and 0x0f]
            sb.append(hi).append(lo)
        }
        return sb.toString()
    }
}
