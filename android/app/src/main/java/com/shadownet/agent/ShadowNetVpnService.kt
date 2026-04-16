package com.shadownet.agent

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import androidx.core.app.NotificationCompat

class ShadowNetVpnService : VpnService() {
    private var vpnInterface: ParcelFileDescriptor? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> startVpn()
            ACTION_STOP -> stopVpn()
        }
        return START_STICKY
    }

    override fun onDestroy() {
        stopVpn()
        super.onDestroy()
    }

    private fun startVpn() {
        if (vpnInterface != null) {
            return
        }
        startForeground(NOTIF_ID, buildNotification("VPN connecting"))

        val builder = Builder()
            .setSession("ShadowNet")
            .setMtu(1400)
            .addAddress("10.0.0.2", 32)
            .addDnsServer("1.1.1.1")
            .addRoute("0.0.0.0", 0)

        vpnInterface = builder.establish()
        if (vpnInterface == null) {
            Logs.add("[vpn] failed to establish tun")
            stopSelf()
            return
        }
        Logs.add("[vpn] tun established")
        startForeground(NOTIF_ID, buildNotification("VPN active"))
    }

    private fun stopVpn() {
        try {
            vpnInterface?.close()
        } catch (_: Exception) {
        } finally {
            vpnInterface = null
            Logs.add("[vpn] stopped")
        }
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
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
            .setContentTitle("ShadowNet VPN")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.stat_sys_vpn_ic)
            .setContentIntent(pi)
            .setOngoing(true)
            .build()
    }

    companion object {
        const val ACTION_START = "com.shadownet.agent.vpn.START"
        const val ACTION_STOP = "com.shadownet.agent.vpn.STOP"
        private const val CHANNEL_ID = "shadownet_vpn"
        private const val NOTIF_ID = 1101
    }
}

