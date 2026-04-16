package com.shadownet.agent

import android.content.Context
import org.json.JSONObject

data class UiState(
    val status: String = "Disconnected",
    val currentProfile: String = "-",
    val lastAction: String = "-",
    val lastError: String = ""
)

class StateRepository(context: Context) {
    private val prefs = context.getSharedPreferences("shadownet_state", Context.MODE_PRIVATE)

    fun save(state: UiState) {
        prefs.edit()
            .putString("status", state.status)
            .putString("profile", state.currentProfile)
            .putString("action", state.lastAction)
            .putString("error", state.lastError)
            .apply()
    }

    fun load(): UiState {
        return UiState(
            status = prefs.getString("status", "Disconnected") ?: "Disconnected",
            currentProfile = prefs.getString("profile", "-") ?: "-",
            lastAction = prefs.getString("action", "-") ?: "-",
            lastError = prefs.getString("error", "") ?: "",
        )
    }

    fun fromBridgeStatus(raw: String): UiState {
        return try {
            val obj = JSONObject(raw)
            val running = obj.optBoolean("running", false)
            UiState(
                status = if (running) "Connected" else "Disconnected",
                currentProfile = obj.optString("current_profile", "-"),
                lastAction = obj.optString("last_action", "-"),
                lastError = obj.optString("last_error", ""),
            )
        } catch (_: Exception) {
            UiState(status = "Error", lastError = "Invalid bridge status")
        }
    }
}

