package com.sunlionet.agent

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.net.ConnectivityManager
import android.net.Network
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.File

class AgentService : Service() {
    private val scope = CoroutineScope(Dispatchers.IO)
    private var monitorJob: Job? = null
    private lateinit var controller: SingBoxController
    private lateinit var repo: StateRepository
    private lateinit var secure: SecureStore
    private var netCallback: ConnectivityManager.NetworkCallback? = null
    private var restartAttempts = 0
    private var probeFailureCount = 0
    private var bridgeFailureCount = 0
    private var lastProbeAtMs: Long = 0L
    private var lastBatteryRestrictedAtMs: Long = 0L
    private var lastRecoveryAtMs: Long = 0L

    override fun onCreate() {
        super.onCreate()
        RuntimeSignals.init(this)
        controller = SingBoxController(this)
        repo = StateRepository(this)
        secure = SecureStore(this)
        secure.ensureDefaultTrustAnchors()
        registerNetworkSwitchMonitor()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> stopAgent()
            ACTION_IMPORT -> {
                val path = intent.getStringExtra(EXTRA_BUNDLE_PATH).orEmpty()
                importBundle(path)
            }
            else -> {
                if (secure.isDesiredConnected()) {
                    startAgent()
                } else {
                    stopSelf()
                }
            }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        unregisterNetworkSwitchMonitor()
        stopAgent(clearUi = false)
        super.onDestroy()
    }

    private fun startAgent() {
        startForeground(NOTIF_ID, buildNotification("Starting agent"))
        prepareDirs()
        Bridge.startAgent(this, usePi = false).onFailure {
            val msg = it.message ?: "start failed"
            Logs.e("agent", msg)
            val mapped = mapBridgeErrorToUi(msg)
            secure.setDesiredConnected(false)
            repo.save(
                UiState(
                    status = "Error",
                    lastError = mapped.first,
                    lastErrorDetails = mapped.second,
                ),
            )
            stopVpnService()
            stopAgent(clearUi = false)
            return
        }
        Logs.i("agent", "started")
        monitorJob?.cancel()
        monitorJob = scope.launch {
            while (isActive) {
                if (!secure.isDesiredConnected()) {
                    stopAgent()
                    return@launch
                }
                val raw = Bridge.getStatus()
                val ui = repo.fromBridgeStatus(raw)
                val merged = mergeBridgeState(ui)
                repo.save(merged)

                if (merged.lastErrorDetails.isNotBlank()) {
                    Logs.w("agent", merged.lastErrorDetails)
                } else if (merged.lastError.isNotBlank()) {
                    Logs.w("agent", merged.lastError)
                }

                if (merged.status == "Error" && merged.lastError.isNotBlank()) {
                    secure.setDesiredConnected(false)
                    stopVpnService()
                    stopAgent(clearUi = false)
                    return@launch
                }

                val configPath = File(filesDir, "runtime/config.json").absolutePath
                val cfgExists = File(configPath).exists()
                if (cfgExists && !controller.isRunning()) {
                    if (restartAttempts < 3) {
                        restartAttempts++
                        controller.start(configPath).onFailure {
                            val msg = it.message ?: "sing-box start failed"
                            Logs.e("agent", "sing-box start failed: $msg")
                            secure.setDesiredConnected(false)
                            repo.save(
                                merged.copy(
                                    status = "Error",
                                    lastAction = "Runtime unavailable",
                                    lastError = "Runtime unavailable",
                                    lastErrorDetails = msg,
                                ),
                            )
                            stopVpnService()
                            stopAgent(clearUi = false)
                        }
                    } else {
                        Logs.w("agent", "restart limit reached")
                    }
                } else if (controller.isRunning()) {
                    restartAttempts = 0
                }

                if (controller.isRunning()) {
                    val now = System.currentTimeMillis()
                    if (now - lastProbeAtMs >= 30_000) {
                        lastProbeAtMs = now
                        repo.save(merged.copy(lastAction = "Testing connection…"))
                        Logs.i("connection", "testing url=https://example.com")
                        val pr = ConnectionProbe.probeHttpViaVpn(this@AgentService, "https://example.com", timeoutMs = 10_000)
                        if (pr.status == "ok") {
                            probeFailureCount = 0
                            bridgeFailureCount = 0
                            repo.save(
                                merged.copy(
                                    status = "Connected",
                                    lastAction = "Connected",
                                    lastError = "",
                                    lastErrorDetails = "",
                                ),
                            )
                            Logs.i("connection", "success http=${pr.httpStatus ?: 0}")
                        } else {
                            probeFailureCount++
                            RuntimeSignals.onConnectionFailure(
                                reason = pr.reason,
                                retryCount = probeFailureCount,
                                success = false,
                            )
                            val mapped = mapProbeFailureToUi(pr.reason, pr.error.orEmpty())
                            val recovering = probeFailureCount < 3
                            repo.save(
                                merged.copy(
                                    status = "Connecting",
                                    lastAction = if (recovering) "Connection failed. Retrying…" else "Connection failed",
                                    lastError = if (recovering) "" else mapped.first,
                                    lastErrorDetails = if (recovering) "" else mapped.second,
                                ),
                            )
                            Logs.w("connection", "failed reason=${pr.reason} err=${pr.error ?: ""}")
                            attemptRecovery(configPath, probeFailureCount)
                        }
                    }
                }

                delay(nextDelayMs())
            }
        }
    }

    private fun stopAgent(clearUi: Boolean = true) {
        monitorJob?.cancel()
        monitorJob = null
        Bridge.stopAgent()
        controller.stop()
        if (clearUi) {
            repo.save(UiState(status = "Disconnected", currentProfile = "-", lastAction = "stopped"))
        }
        Logs.i("agent", "stopped")
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun stopVpnService() {
        runCatching {
            startService(Intent(this, SUNLIONETVpnService::class.java).apply { action = SUNLIONETVpnService.ACTION_STOP })
        }
    }

    private fun importBundle(path: String) {
        if (path.isBlank()) {
            Logs.w("agent", "import failed: empty path")
            return
        }
        repo.save(UiState(status = "Disconnected", currentProfile = "-", lastAction = "Importing configuration…"))
        Bridge.importBundle(path).onFailure {
            val msg = it.message ?: "import failed"
            Logs.e("agent", "import failed: $msg")
            val mapped = mapBridgeErrorToUi(msg)
            repo.save(
                UiState(
                    status = "Error",
                    currentProfile = "-",
                    lastAction = "Import failed",
                    lastError = mapped.first,
                    lastErrorDetails = mapped.second,
                ),
            )
        }.onSuccess {
            Logs.i("agent", "import success")
            repo.save(
                UiState(
                    status = "Disconnected",
                    currentProfile = "-",
                    lastAction = "Ready to connect",
                    lastError = "",
                    lastErrorDetails = "",
                ),
            )
        }
    }

    private fun mergeBridgeState(ui: UiState): UiState {
        val desired = secure.isDesiredConnected()
        if (!desired) {
            probeFailureCount = 0
            bridgeFailureCount = 0
            val cleared = ui.copy(status = "Disconnected", lastError = "", lastErrorDetails = "")
            return cleared
        }
        if (ui.lastError.isNotBlank()) {
            bridgeFailureCount++
            val mapped = mapBridgeErrorToUi(ui.lastError)
            val isConfigError = mapped.first == getString(R.string.error_config_missing) ||
                mapped.first == getString(R.string.error_config_invalid)
            val recovering = !isConfigError && bridgeFailureCount < 3
            if (recovering) {
                attemptRecovery(File(filesDir, "runtime/config.json").absolutePath, bridgeFailureCount)
                return ui.copy(
                    status = "Connecting",
                    lastAction = "Retrying…",
                    lastError = "",
                    lastErrorDetails = "",
                )
            }
            return ui.copy(status = "Error", lastError = mapped.first, lastErrorDetails = mapped.second)
        }
        bridgeFailureCount = 0
        if (ui.status != "Connected") {
            return ui.copy(status = "Connecting", lastError = "", lastErrorDetails = "")
        }
        return ui.copy(status = "Connected", lastError = "", lastErrorDetails = "")
    }

    private fun mapBridgeErrorToUi(raw: String): Pair<String, String> {
        val msg = raw.trim()
        val lower = msg.lowercase()
        if (lower.contains("classnotfoundexception") || lower.contains("com.sunlionet.mobile.mobile")) {
            return "Native runtime unavailable" to msg
        }
        if (lower.contains("no profiles available")) {
            return getString(R.string.error_config_missing) to "No profiles available"
        }
        if (lower.contains("bundle invalid") ||
            lower.contains("signature") ||
            lower.contains("decrypt") ||
            lower.contains("expired") ||
            lower.contains("unknown signer") ||
            lower.contains("replay")
        ) {
            return getString(R.string.error_config_invalid) to msg
        }
        return getString(R.string.status_detail_failed) to msg
    }

    private fun mapProbeFailureToUi(reason: String, details: String): Pair<String, String> {
        return when (reason.uppercase()) {
            "DNS_FAILURE", "DNS_BLOCKED" -> getString(R.string.error_network_blocked) to "DNS failure detected\n\n$details".trim()
            "TLS_BLOCKED" -> getString(R.string.error_network_blocked) to "TLS handshake blocked\n\n$details".trim()
            "TCP_RESET" -> getString(R.string.error_network_blocked) to "Connection reset detected\n\n$details".trim()
            "NO_ROUTE" -> getString(R.string.status_detail_disconnected) to "No route to host\n\n$details".trim()
            "TIMEOUT" -> getString(R.string.error_network_blocked) to "Timeout detected\n\n$details".trim()
            else -> getString(R.string.status_detail_failed) to "Unknown failure\n\n$details".trim()
        }
    }

    private fun attemptRecovery(configPath: String, failureCount: Int) {
        val now = System.currentTimeMillis()
        if (now - lastRecoveryAtMs < 8_000) {
            return
        }
        lastRecoveryAtMs = now
        if (failureCount == 1) {
            runCatching {
                controller.stop()
                if (File(configPath).exists()) {
                    controller.start(configPath)
                }
            }
            return
        }
        if (failureCount == 2) {
            runCatching {
                Bridge.stopAgent()
                Bridge.startAgent(this, usePi = false)
                restartAttempts = 0
            }
        }
    }

    private fun prepareDirs() {
        File(filesDir, "state").mkdirs()
        File(filesDir, "runtime").mkdirs()
        val templatesDir = File(filesDir, "templates")
        templatesDir.mkdirs()
        val defaultTemplates = listOf("reality.json", "hysteria2.json", "tuic.json")
        for (name in defaultTemplates) {
            val target = File(templatesDir, name)
            if (target.exists()) {
                continue
            }
            runCatching {
                assets.open("templates/$name").use { input ->
                    target.outputStream().use { output -> input.copyTo(output) }
                }
            }.onFailure {
                Logs.w("agent", "template missing in assets: $name")
            }
        }
    }

    private fun nextDelayMs(): Long {
        val lowBattery = runCatching {
            val filter = android.content.IntentFilter(android.content.Intent.ACTION_BATTERY_CHANGED)
            val status = registerReceiver(null, filter) ?: return@runCatching false
            val l = status.getIntExtra(android.os.BatteryManager.EXTRA_LEVEL, 100)
            val scale = status.getIntExtra(android.os.BatteryManager.EXTRA_SCALE, 100)
            val pct = if (scale > 0) (l * 100) / scale else 100
            val plugged = status.getIntExtra(android.os.BatteryManager.EXTRA_PLUGGED, 0) != 0
            !plugged && pct <= 15
        }.getOrDefault(false)

        if (lowBattery) {
            val now = System.currentTimeMillis()
            if (now - lastBatteryRestrictedAtMs >= 10 * 60_000L) {
                RuntimeSignals.onRuntimeEvent("BATTERY_RESTRICTED")
                Logs.w("runtime", "battery saver risk: low battery can delay reconnect")
                lastBatteryRestrictedAtMs = now
            }
        }

        return if (lowBattery) 15_000 else 5_000
    }

    private fun registerNetworkSwitchMonitor() {
        val cm = getSystemService(ConnectivityManager::class.java) ?: return
        val callback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                RuntimeSignals.onRuntimeEvent("NETWORK_SWITCH")
                Logs.i("network", "switch detected")
            }
        }
        runCatching {
            cm.registerDefaultNetworkCallback(callback)
            netCallback = callback
        }.onFailure {
            Logs.w("network", "monitor unavailable")
        }
    }

    private fun unregisterNetworkSwitchMonitor() {
        val cm = getSystemService(ConnectivityManager::class.java) ?: return
        val callback = netCallback ?: return
        runCatching { cm.unregisterNetworkCallback(callback) }
        netCallback = null
    }

    private fun buildNotification(text: String): Notification {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                getString(R.string.notif_channel_agent),
                NotificationManager.IMPORTANCE_LOW,
            )
            val mgr = getSystemService(NotificationManager::class.java)
            mgr.createNotificationChannel(channel)
        }
        val intent = Intent(this, MainActivity::class.java)
        val pi = PendingIntent.getActivity(
            this,
            0,
            intent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("SunLionet Agent")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_popup_sync)
            .setContentIntent(pi)
            .setOngoing(true)
            .build()
    }

    companion object {
        const val ACTION_START = "com.sunlionet.agent.agent.START"
        const val ACTION_STOP = "com.sunlionet.agent.agent.STOP"
        const val ACTION_IMPORT = "com.sunlionet.agent.agent.IMPORT"
        const val EXTRA_BUNDLE_PATH = "bundle_path"
        private const val CHANNEL_ID = "SUNLIONET_agent"
        private const val NOTIF_ID = 1102
    }
}
