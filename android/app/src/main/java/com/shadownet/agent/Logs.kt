package com.shadownet.agent

import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.CopyOnWriteArrayList

object Logs {
    private const val maxLines = 200
    private val lines = ArrayDeque<String>()
    private val listeners = CopyOnWriteArrayList<(String) -> Unit>()
    private val ts = SimpleDateFormat("HH:mm:ss", Locale.US)

    @Synchronized
    fun add(msg: String) {
        val line = "${ts.format(Date())} $msg"
        lines.addLast(line)
        while (lines.size > maxLines) {
            lines.removeFirst()
        }
        val content = lines.joinToString("\n")
        listeners.forEach { it(content) }
    }

    @Synchronized
    fun dump(): String = lines.joinToString("\n")

    fun observe(listener: (String) -> Unit) {
        listeners.add(listener)
        listener(dump())
    }

    fun removeObserver(listener: (String) -> Unit) {
        listeners.remove(listener)
    }
}

