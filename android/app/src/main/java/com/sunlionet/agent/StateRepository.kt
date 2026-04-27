package com.sunlionet.agent

import android.content.Context

data class UiState(
    val status: String = "Disconnected",
    val currentProfile: String = "-",
    val lastAction: String = "-",
    val lastError: String = "",
    val lastErrorDetails: String = "",
)

class StateRepository(context: Context) {
    private val prefs = context.getSharedPreferences("SUNLIONET_state", Context.MODE_PRIVATE)

    fun save(state: UiState) {
        prefs.edit()
            .putString("status", state.status)
            .putString("profile", state.currentProfile)
            .putString("action", state.lastAction)
            .putString("error", state.lastError)
            .putString("error_details", state.lastErrorDetails)
            .apply()
    }

    fun load(): UiState {
        return UiState(
            status = prefs.getString("status", "Disconnected") ?: "Disconnected",
            currentProfile = prefs.getString("profile", "-") ?: "-",
            lastAction = prefs.getString("action", "-") ?: "-",
            lastError = prefs.getString("error", "") ?: "",
            lastErrorDetails = prefs.getString("error_details", "") ?: "",
        )
    }

    fun fromBridgeStatus(raw: String): UiState {
        return parseBridgeStatus(raw)
    }

    companion object {
        fun parseBridgeStatus(raw: String): UiState {
            val trimmed = raw.trim()
            if (trimmed.isEmpty()) return UiState(status = "Error", lastError = "Invalid bridge status")
            if (!trimmed.startsWith("{") || !trimmed.endsWith("}")) return UiState(status = "Error", lastError = "Invalid bridge status")

            val runningValue = Regex(""""running"\s*:\s*(true|false|1|0)""", RegexOption.IGNORE_CASE)
                .find(trimmed)
                ?.groupValues
                ?.getOrNull(1)
                ?.lowercase()
            val running = runningValue == "true" || runningValue == "1"

            fun readJsonString(key: String): String? {
                val m = Regex(""""${Regex.escape(key)}"\s*:\s*"((?:\\.|[^"])*)"""").find(trimmed) ?: return null
                return m.groupValues.getOrNull(1)?.let { v ->
                    v.replace("\\\\", "\\")
                        .replace("\\\"", "\"")
                        .replace("\\n", "\n")
                        .replace("\\r", "\r")
                        .replace("\\t", "\t")
                }
            }

            val profile = readJsonString("current_profile") ?: "-"
            val action = readJsonString("last_action") ?: "-"
            val error = readJsonString("last_error") ?: ""

            return UiState(
                status = if (running) "Connected" else "Disconnected",
                currentProfile = profile,
                lastAction = action,
                lastError = error,
                lastErrorDetails = "",
            )
        }
    }
}
