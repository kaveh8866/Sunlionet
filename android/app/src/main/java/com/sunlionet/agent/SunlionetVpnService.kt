package com.sunlionet.agent

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.ConnectivityManager
import android.net.Network
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.system.OsConstants
import androidx.core.app.NotificationCompat

class SunlionetVpnService : VpnService() {
    private var vpnInterface: ParcelFileDescriptor? = null
    private lateinit var repo: StateRepository
    private lateinit var secure: SecureStore
    private var state: State = State.IDLE
    private var netCallback: ConnectivityManager.NetworkCallback? = null

    enum class State {
        IDLE,
        STARTING,
        HOLDING,
        RUNNING,
        ERROR,
        STOPPED,
    }

    override fun onCreate() {
        super.onCreate()
        RuntimeSignals.init(this)
        repo = StateRepository(this)
        secure = SecureStore(this)
        registerNetworkWatchdog()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> startVpn()
            ACTION_HOLD -> holdVpn(intent.getStringExtra(EXTRA_HOLD_REASON).orEmpty().ifBlank { "runtime transition" })
            ACTION_STOP -> stopVpn()
            else -> {
                if (secure.isDesiredConnected()) {
                    startVpn()
                } else {
                    stopSelf()
                }
            }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        unregisterNetworkWatchdog()
        stopVpn()
        super.onDestroy()
    }

    override fun onRevoke() {
        Logs.w("vpn", "revoked by system")
        RuntimeSignals.onRuntimeEvent("VPN_DISCONNECT")
        secure.setDesiredConnected(false)
        stopVpn()
        super.onRevoke()
    }

    private fun startVpn() {
        if (vpnInterface != null) {
            return
        }
        RuntimeSignals.onRuntimeEvent("VPN_RESTART")
        if (!secure.isDesiredConnected()) {
            stopSelf()
            return
        }
        state = State.STARTING
        repo.save(UiState(status = "Connecting", currentProfile = repo.load().currentProfile, lastAction = "vpn starting"))
        startForeground(NOTIF_ID, buildNotification("VPN connecting"))

        val builder = Builder()
            .setSession("SunLionet")
            .setMtu(VpnLeakPolicy.MTU)
            .addAddress(VpnLeakPolicy.IPV4_ADDRESS, VpnLeakPolicy.IPV4_PREFIX)
            .addAddress(VpnLeakPolicy.IPV6_ADDRESS, VpnLeakPolicy.IPV6_PREFIX)
            .addDnsServer(VpnLeakPolicy.IPV4_DNS)
            .addDnsServer(VpnLeakPolicy.IPV6_DNS)
            .addRoute("0.0.0.0", 0)
            .addRoute("::", 0)
            .allowFamily(OsConstants.AF_INET)
            .allowFamily(OsConstants.AF_INET6)

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
            builder.setUnderlyingNetworks(emptyArray())
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            builder.setMetered(false)
            builder.setBlocking(true)
        }

        vpnInterface = builder.establish()
        if (vpnInterface == null) {
            state = State.ERROR
            Logs.e("vpn", "failed to establish tun")
            repo.save(UiState(status = "Error", lastError = "VPN failed to establish"))
            stopSelf()
            return
        }
        state = State.RUNNING
        Logs.i("vpn", "tun established")
        startForeground(NOTIF_ID, buildNotification("VPN active"))
    }

    private fun holdVpn(reason: String) {
        if (!VpnLeakPolicy.shouldHoldTunnel(secure.isDesiredConnected(), vpnInterface != null)) {
            return
        }
        state = State.HOLDING
        Logs.w("vpn", "holding tunnel: $reason")
        repo.save(UiState(status = "Connecting", currentProfile = repo.load().currentProfile, lastAction = "network changed, holding tunnel"))
        startForeground(NOTIF_ID, buildNotification("VPN holding secure route"))
    }

    private fun stopVpn() {
        val wasRunning = vpnInterface != null
        try {
            vpnInterface?.close()
        } catch (_: Exception) {
        } finally {
            vpnInterface = null
            state = State.STOPPED
            Logs.i("vpn", "stopped")
            if (wasRunning && secure.isDesiredConnected()) {
                RuntimeSignals.onRuntimeEvent("VPN_DISCONNECT")
            }
        }
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun registerNetworkWatchdog() {
        val cm = getSystemService(ConnectivityManager::class.java) ?: return
        val cb = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                holdVpn("network available")
                restartAgentIfDesired()
            }

            override fun onLost(network: Network) {
                holdVpn("network lost")
            }
        }
        runCatching {
            cm.registerDefaultNetworkCallback(cb)
            netCallback = cb
        }.onFailure {
            Logs.w("vpn", "network watchdog unavailable")
        }
    }

    private fun unregisterNetworkWatchdog() {
        val cb = netCallback ?: return
        val cm = getSystemService(ConnectivityManager::class.java) ?: return
        runCatching { cm.unregisterNetworkCallback(cb) }
        netCallback = null
    }

    private fun restartAgentIfDesired() {
        if (!secure.isDesiredConnected()) {
            return
        }
        runCatching {
            startForegroundService(Intent(this, AgentService::class.java).apply { action = AgentService.ACTION_START })
        }
    }

    private fun buildNotification(text: String): Notification {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                getString(R.string.notif_channel_vpn),
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
            .setContentTitle("SunLionet VPN")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .setContentIntent(pi)
            .setOngoing(true)
            .build()
    }

    companion object {
        const val ACTION_START = "com.sunlionet.agent.vpn.START"
        const val ACTION_HOLD = "com.sunlionet.agent.vpn.HOLD"
        const val ACTION_STOP = "com.sunlionet.agent.vpn.STOP"
        const val EXTRA_HOLD_REASON = "hold_reason"
        private const val CHANNEL_ID = "SUNLIONET_vpn"
        private const val NOTIF_ID = 1101
    }
}
