package com.shadownet.agent

import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.CopyOnWriteArrayList
import java.util.regex.Pattern

object Logs {
    private const val maxLines = 200
    private val lines = ArrayDeque<String>()
    private val listeners = CopyOnWriteArrayList<(String) -> Unit>()
    private val ts = SimpleDateFormat("HH:mm:ss", Locale.US)

    enum class Level {
        INFO,
        WARN,
        ERROR,
    }

    private val ipPattern = Pattern.compile("\\b(?:\\d{1,3}\\.){3}\\d{1,3}\\b")
    private val secretPattern = Pattern.compile("(?i)(age-secret-key-[a-z0-9]+|[a-f0-9]{64})")

    @Synchronized
    fun add(msg: String) {
        if (!BuildConfig.DEBUG) {
            return
        }
        val line = "${ts.format(Date())} ${sanitize(msg)}"
        lines.addLast(line)
        while (lines.size > maxLines) {
            lines.removeFirst()
        }
        val content = lines.joinToString("\n")
        listeners.forEach { it(content) }
    }

    @Synchronized
    fun dump(): String = lines.joinToString("\n")

    fun i(component: String, msg: String) {
        if (!BuildConfig.DEBUG) return
        add("[${component.lowercase()}][${Level.INFO}] $msg")
    }

    fun w(component: String, msg: String) {
        if (!BuildConfig.DEBUG) return
        add("[${component.lowercase()}][${Level.WARN}] $msg")
    }

    fun e(component: String, msg: String) {
        val line = "[${component.lowercase()}][${Level.ERROR}] $msg"
        if (BuildConfig.DEBUG) {
            add(line)
            return
        }
        // In release builds, keep only bounded error summaries for local UI feedback.
        val safe = "${ts.format(Date())} [${component.lowercase()}][${Level.ERROR}] ${sanitize(msg)}"
        synchronized(this) {
            lines.addLast(safe)
            while (lines.size > maxLines) {
                lines.removeFirst()
            }
        }
    }

    fun observe(listener: (String) -> Unit) {
        listeners.add(listener)
        listener(dump())
    }

    fun removeObserver(listener: (String) -> Unit) {
        listeners.remove(listener)
    }

    private fun sanitize(input: String): String {
        var out = input.trim()
        out = ipPattern.matcher(out).replaceAll("x.x.x.x")
        out = secretPattern.matcher(out).replaceAll("[redacted]")
        if (out.length > 240) {
            out = out.take(240) + "…"
        }
        return out
    }
}
