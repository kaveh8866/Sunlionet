package com.shadownet.agent

import android.content.Context
import org.json.JSONObject
import java.io.File

object Bridge {
    private val classCandidates = listOf(
        "go.mobilebridge.Mobilebridge",
        "go.shadownet.mobilebridge.Mobilebridge",
        "mobilebridge.Mobilebridge",
    )

    private fun bridgeClass(): Class<*> {
        for (name in classCandidates) {
            try {
                return Class.forName(name)
            } catch (_: Throwable) {
            }
        }
        throw IllegalStateException("Go bridge class not found. Build gomobile binding for pkg/mobilebridge.")
    }

    fun startAgent(context: Context, usePi: Boolean = false) {
        val cfg = JSONObject().apply {
            put("state_dir", File(context.filesDir, "state").absolutePath)
            put("master_key", "0123456789abcdef0123456789abcdef")
            put("templates_dir", File(context.filesDir, "templates").absolutePath)
            put("poll_interval_sec", 20)
            put("config_path", File(context.filesDir, "runtime/config.json").absolutePath)
            put("use_pi", usePi)
            put("pi_timeout_ms", 1200)
            put("pi_command", "pi")
        }.toString()

        val clazz = bridgeClass()
        val method = clazz.getMethod("StartAgent", String::class.java)
        method.invoke(null, cfg)
        Logs.add("[bridge] start agent")
    }

    fun stopAgent() {
        val clazz = bridgeClass()
        val method = clazz.getMethod("StopAgent")
        method.invoke(null)
        Logs.add("[bridge] stop agent")
    }

    fun importBundle(path: String): Result<Unit> {
        return runCatching {
            val clazz = bridgeClass()
            val method = clazz.getMethod("ImportBundle", String::class.java)
            method.invoke(null, path)
            Logs.add("[bridge] import bundle: $path")
        }
    }

    fun getStatus(): String {
        return try {
            val clazz = bridgeClass()
            val method = clazz.getMethod("GetStatus")
            method.invoke(null) as? String ?: """{"running":false,"last_error":"empty status"}"""
        } catch (e: Throwable) {
            """{"running":false,"last_error":"${e.message ?: "bridge unavailable"}"}"""
        }
    }
}

