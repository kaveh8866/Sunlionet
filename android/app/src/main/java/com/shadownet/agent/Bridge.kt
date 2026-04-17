package com.shadownet.agent

import android.content.Context
import com.shadownet.mobile.Mobile
import org.json.JSONObject
import java.io.File

object Bridge {
    fun startAgent(context: Context, usePi: Boolean = false): Result<Unit> = runCatching {
        val secure = SecureStore(context)
        val cfg = JSONObject().apply {
            put("state_dir", File(context.filesDir, "state").absolutePath)
            put("master_key", secure.getOrCreateMasterKeyB64Url())
            put("templates_dir", File(context.filesDir, "templates").absolutePath)
            put("poll_interval_sec", 20)
            put("config_path", File(context.filesDir, "runtime/config.json").absolutePath)
            put("use_pi", usePi)
            put("pi_timeout_ms", 1200)
            put("pi_command", "pi")
            put("trusted_signer_pub_b64url", secure.getTrustedSignerKeysCSV())
            put("age_identity", secure.getOrCreateAgeIdentity())
        }.toString()
        Mobile.StartAgent(cfg)
        Logs.i("bridge", "agent started")
    }

    fun stopAgent(): Result<Unit> = runCatching {
        Mobile.StopAgent()
        Logs.i("bridge", "agent stopped")
    }

    fun importBundle(path: String): Result<Unit> {
        return runCatching {
            Mobile.ImportBundle(path)
            Logs.i("bridge", "import bundle: $path")
        }
    }

    fun getStatus(): String {
        return try {
            Mobile.GetStatus()
        } catch (e: Throwable) {
            """{"running":false,"last_error":"${e.message ?: "bridge unavailable"}"}"""
        }
    }
}
