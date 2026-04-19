package com.shadownet.agent

import android.content.Context
import org.json.JSONObject
import java.io.File

object Bridge {
    private val mobileClassNames = listOf(
        "com.shadownet.mobile.Mobile",
        "com.shadownet.mobile.mobile.Mobile",
    )

    private fun mobileClass(): Class<*> {
        var last: Throwable? = null
        for (name in mobileClassNames) {
            try {
                return Class.forName(name)
            } catch (t: Throwable) {
                last = t
            }
        }
        throw last ?: ClassNotFoundException(mobileClassNames.firstOrNull() ?: "com.shadownet.mobile.Mobile")
    }

    private fun callMobile(methodName: String, argTypes: Array<Class<*>>, args: Array<Any?>) {
        val cls = mobileClass()
        val candidates = listOf(
            methodName,
            methodName.replaceFirstChar { it.lowercase() },
        ).distinct()
        var last: Throwable? = null
        for (name in candidates) {
            try {
                val method = cls.getMethod(name, *argTypes)
                method.invoke(null, *args)
                return
            } catch (t: Throwable) {
                last = t
            }
        }
        throw last ?: NoSuchMethodException("${cls.name}.$methodName")
    }

    private fun callMobileString(methodName: String): String {
        val cls = mobileClass()
        val candidates = listOf(
            methodName,
            methodName.replaceFirstChar { it.lowercase() },
        ).distinct()
        var last: Throwable? = null
        for (name in candidates) {
            try {
                val method = cls.getMethod(name)
                val out = method.invoke(null)
                return out as? String ?: ""
            } catch (t: Throwable) {
                last = t
            }
        }
        throw last ?: NoSuchMethodException("${cls.name}.$methodName")
    }

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
        callMobile("StartAgent", arrayOf(String::class.java), arrayOf(cfg))
        Logs.i("bridge", "agent started")
    }

    fun stopAgent(): Result<Unit> = runCatching {
        callMobile("StopAgent", emptyArray(), emptyArray())
        Logs.i("bridge", "agent stopped")
    }

    fun importBundle(path: String): Result<Unit> {
        return runCatching {
            callMobile("ImportBundle", arrayOf(String::class.java), arrayOf(path))
            Logs.i("bridge", "import bundle: $path")
        }
    }

    fun getStatus(): String {
        return try {
            callMobileString("GetStatus")
        } catch (e: Throwable) {
            """{"running":false,"last_error":"${e.message ?: "bridge unavailable"}"}"""
        }
    }
}
