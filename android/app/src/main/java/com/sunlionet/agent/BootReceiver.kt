package com.sunlionet.agent

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        RuntimeSignals.init(context)
        RuntimeSignals.onRuntimeEvent("APP_KILLED")
        val secure = SecureStore(context)
        if (!secure.isDesiredConnected()) {
            return
        }
        val vpnIntent = Intent(context, SUNLIONETVpnService::class.java).apply {
            action = SUNLIONETVpnService.ACTION_START
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
