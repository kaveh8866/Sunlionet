package com.shadownet.agent

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
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
    private var restartAttempts = 0
    private var lastProbeAtMs: Long = 0L

    override fun onCreate() {
        super.onCreate()
        controller = SingBoxController(this)
        repo = StateRepository(this)
        secure = SecureStore(this)
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
        stopAgent()
        super.onDestroy()
    }

    private fun startAgent() {
        startForeground(NOTIF_ID, buildNotification("Starting agent"))
        prepareDirs()
        Bridge.startAgent(this, usePi = false).onFailure {
            val msg = it.message ?: "start failed"
            Logs.e("agent", msg)
            repo.save(UiState(status = "Error", lastError = msg))
            stopAgent()
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
                repo.save(ui)

                if (ui.lastError.isNotBlank()) {
                    Logs.w("agent", ui.lastError)
                }

                val configPath = File(filesDir, "runtime/config.json").absolutePath
                val cfgExists = File(configPath).exists()
                if (cfgExists && !controller.isRunning()) {
                    if (restartAttempts < 3) {
                        restartAttempts++
                        controller.start(configPath).onFailure {
                            Logs.e("agent", "sing-box start failed: ${it.message}")
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
                        repo.save(ui.copy(lastAction = "[connection] testing..."))
                        Logs.i("connection", "testing url=https://example.com")
                        val pr = ConnectionProbe.probeHttpViaVpn(this@AgentService, "https://example.com", timeoutMs = 10_000)
                        if (pr.status == "ok") {
                            repo.save(ui.copy(lastAction = "[connection] success http=${pr.httpStatus ?: 0}"))
                            Logs.i("connection", "success http=${pr.httpStatus ?: 0}")
                        } else {
                            repo.save(ui.copy(lastAction = "[connection] failed: ${pr.reason}", lastError = pr.error ?: ""))
                            Logs.w("connection", "failed reason=${pr.reason} err=${pr.error ?: ""}")
                        }
                    }
                }

                delay(nextDelayMs())
            }
        }
    }

    private fun stopAgent() {
        monitorJob?.cancel()
        monitorJob = null
        Bridge.stopAgent()
        controller.stop()
        repo.save(UiState(status = "Disconnected", currentProfile = "-", lastAction = "stopped"))
        Logs.i("agent", "stopped")
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun importBundle(path: String) {
        if (path.isBlank()) {
            Logs.w("agent", "import failed: empty path")
            return
        }
        Bridge.importBundle(path).onFailure {
            Logs.e("agent", "import failed: ${it.message}")
        }.onSuccess {
            Logs.i("agent", "import success")
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
        val level = runCatching {
            val filter = android.content.IntentFilter(android.content.Intent.ACTION_BATTERY_CHANGED)
            val status = registerReceiver(null, filter) ?: return@runCatching 100
            val l = status.getIntExtra(android.os.BatteryManager.EXTRA_LEVEL, 100)
            val scale = status.getIntExtra(android.os.BatteryManager.EXTRA_SCALE, 100)
            val pct = if (scale > 0) (l * 100) / scale else 100
            val plugged = status.getIntExtra(android.os.BatteryManager.EXTRA_PLUGGED, 0) != 0
            if (!plugged && pct <= 15) 15 else 100
        }.getOrDefault(100)
        return if (level <= 15) 15_000 else 5_000
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
            .setContentTitle("ShadowNet Agent")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_popup_sync)
            .setContentIntent(pi)
            .setOngoing(true)
            .build()
    }

    companion object {
        const val ACTION_START = "com.shadownet.agent.agent.START"
        const val ACTION_STOP = "com.shadownet.agent.agent.STOP"
        const val ACTION_IMPORT = "com.shadownet.agent.agent.IMPORT"
        const val EXTRA_BUNDLE_PATH = "bundle_path"
        private const val CHANNEL_ID = "shadownet_agent"
        private const val NOTIF_ID = 1102
    }
}
