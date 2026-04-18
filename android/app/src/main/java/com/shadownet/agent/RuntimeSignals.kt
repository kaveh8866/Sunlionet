package com.shadownet.agent

import android.content.Context

object RuntimeSignals {
    @Volatile
    private var store: DiagnosticsStore? = null
    @Volatile
    private var crashHandlerInstalled = false

    fun init(context: Context) {
        if (store != null) {
            return
        }
        synchronized(this) {
            if (store == null) {
                store = DiagnosticsStore(context.applicationContext)
            }
        }
        installCrashHandler()
    }

    fun diagnosticsStoreOrNull(): DiagnosticsStore? = store

    fun onConnectionFailure(reason: String, retryCount: Int, success: Boolean) {
        store?.recordConnectionFailure(reason = reason, retryCount = retryCount, success = success)
    }

    fun onRuntimeEvent(eventName: String) {
        store?.recordRuntimeEvent(eventName)
    }

    fun onError(component: String, reason: String) {
        store?.recordError(component = component, reason = reason)
    }

    private fun installCrashHandler() {
        if (crashHandlerInstalled) {
            return
        }
        synchronized(this) {
            if (crashHandlerInstalled) {
                return
            }
            val previous = Thread.getDefaultUncaughtExceptionHandler()
            Thread.setDefaultUncaughtExceptionHandler { thread, throwable ->
                runCatching {
                    onRuntimeEvent("APP_CRASH")
                    onError("app", throwable.javaClass.simpleName)
                }
                previous?.uncaughtException(thread, throwable)
            }
            crashHandlerInstalled = true
        }
    }
}
