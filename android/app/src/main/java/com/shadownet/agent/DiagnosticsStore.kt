package com.shadownet.agent

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

class DiagnosticsStore(private val context: Context) {
    private val prefs = context.getSharedPreferences("shadownet_diag", Context.MODE_PRIVATE)
    private val ts = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.US)
    private val allowedReasons = setOf(
        "DNS_FAILURE",
        "DNS_BLOCKED",
        "TCP_RESET",
        "TLS_BLOCKED",
        "TIMEOUT",
        "NO_ROUTE",
        "UNKNOWN",
    )
    private val allowedRuntimeEvents = setOf(
        "APP_KILLED",
        "APP_CRASH",
        "VPN_RESTART",
        "VPN_DISCONNECT",
        "NETWORK_SWITCH",
        "BATTERY_RESTRICTED",
    )

    fun isDiagnosticsEnabled(): Boolean = prefs.getBoolean("diagnostics_enabled", false)

    fun setDiagnosticsEnabled(enabled: Boolean) {
        prefs.edit().putBoolean("diagnostics_enabled", enabled).apply()
    }

    @Synchronized
    fun recordConnectionFailure(reason: String, retryCount: Int, success: Boolean) {
        if (!isDiagnosticsEnabled()) {
            return
        }
        val event = JSONObject()
            .put("event", "connection_fail")
            .put("reason", normalizeReason(reason))
            .put("retry_count", retryCount.coerceAtLeast(0))
            .put("success", success)
            .put("ts", ts.format(Date()))
        appendEvent(event)
    }

    @Synchronized
    fun recordRuntimeEvent(eventName: String) {
        if (!isDiagnosticsEnabled()) {
            return
        }
        val safeEvent = if (allowedRuntimeEvents.contains(eventName)) eventName else "UNKNOWN"
        appendEvent(
            JSONObject()
                .put("event", safeEvent)
                .put("ts", ts.format(Date())),
        )
    }

    @Synchronized
    fun recordError(component: String, reason: String) {
        val err = JSONObject()
            .put("component", component.lowercase())
            .put("reason", normalizeReason(reason))
            .put("ts", ts.format(Date()))
        val file = File(context.filesDir, "last_errors.json")
        val arr = readArray(file).put(err)
        while (arr.length() > 25) {
            arr.remove(0)
        }
        file.writeText(arr.toString())
    }

    fun lastErrorLabel(): String {
        val file = File(context.filesDir, "last_errors.json")
        val arr = readArray(file)
        if (arr.length() == 0) {
            return "-"
        }
        val last = arr.optJSONObject(arr.length() - 1) ?: return "-"
        return last.optString("reason", "UNKNOWN")
    }

    @Synchronized
    fun exportLogsJson(logDump: String): File {
        val outDir = File(context.filesDir, "exports").apply { mkdirs() }
        val out = File(outDir, "logs.json")
        val payload = JSONObject()
            .put("schema", "shadownet.logs.v1")
            .put("version", BuildConfig.APP_VERSION_LABEL)
            .put("tester_mode", BuildConfig.TESTER_MODE)
            .put("share_anonymous_diagnostics", isDiagnosticsEnabled())
            .put("generated_at", ts.format(Date()))
            .put("events", readArray(File(context.filesDir, "diagnostics_events.json")))
            .put("last_errors", readArray(File(context.filesDir, "last_errors.json")))
            .put("logs", JSONArray().apply {
                logDump.lines().filter { it.isNotBlank() }.takeLast(200).forEach { put(it) }
            })
        out.writeText(payload.toString(2))
        return out
    }

    private fun normalizeReason(reason: String): String {
        val upper = reason.uppercase(Locale.US).trim()
        return if (allowedReasons.contains(upper)) upper else "UNKNOWN"
    }

    private fun appendEvent(event: JSONObject) {
        val file = File(context.filesDir, "diagnostics_events.json")
        val arr = readArray(file).put(event)
        while (arr.length() > 200) {
            arr.remove(0)
        }
        file.writeText(arr.toString())
    }

    private fun readArray(file: File): JSONArray {
        if (!file.exists()) {
            return JSONArray()
        }
        return runCatching { JSONArray(file.readText()) }.getOrElse { JSONArray() }
    }
}
