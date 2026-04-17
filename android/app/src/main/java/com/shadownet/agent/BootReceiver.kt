package com.shadownet.agent

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        val secure = SecureStore(context)
        if (!secure.isDesiredConnected()) {
            return
        }
        val vpnIntent = Intent(context, ShadowNetVpnService::class.java).apply {
            action = ShadowNetVpnService.ACTION_START
        }
        val agentIntent = Intent(context, AgentService::class.java).apply {
            action = AgentService.ACTION_START
        }
        runCatching {
            context.startForegroundService(vpnIntent)
            context.startForegroundService(agentIntent)
            Logs.i("boot", "restart requested action=${intent?.action.orEmpty()}")
        }.onFailure {
            Logs.e("boot", "restart failed: ${it.message}")
        }
    }
}

