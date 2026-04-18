package com.shadownet.agent

import android.content.Context
import org.json.JSONObject
import java.io.File

object Bridge {
    private fun mobileClass(): Class<*> = Class.forName("com.shadownet.mobile.Mobile")

    private fun callMobile(methodName: String, argTypes: Array<Class<*>>, args: Array<Any?>) {
        val cls = mobileClass()
        val method = cls.getMethod(methodName, *argTypes)
        method.invoke(null, *args)
    }

    private fun callMobileString(methodName: String): String {
        val cls = mobileClass()
        val method = cls.getMethod(methodName)
        val out = method.invoke(null)
        return out as? String ?: ""
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
