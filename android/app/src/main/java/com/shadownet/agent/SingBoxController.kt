package com.shadownet.agent

import android.content.Context
import java.io.File
import java.util.concurrent.Executors

class SingBoxController(private val context: Context) {
    private var process: Process? = null
    private val ioExec = Executors.newSingleThreadExecutor()

    private fun binaryFile(): File = File(context.filesDir, "bin/sing-box")

    private fun assetNameForAbi(): String {
        val abi = android.os.Build.SUPPORTED_ABIS.firstOrNull().orEmpty()
        return when {
            abi.contains("arm64") -> "sing-box/arm64-v8a/sing-box"
            abi.contains("armeabi") -> "sing-box/armeabi-v7a/sing-box"
            else -> "sing-box/arm64-v8a/sing-box"
        }
    }

    fun ensureBinary(): Result<File> = runCatching {
        val out = binaryFile()
        if (!out.exists()) {
            out.parentFile?.mkdirs()
            context.assets.open(assetNameForAbi()).use { input ->
                out.outputStream().use { output -> input.copyTo(output) }
            }
        }
        if (!out.setExecutable(true)) {
            throw IllegalStateException("failed to mark sing-box executable")
        }
        out
    }

    @Synchronized
    fun start(configPath: String): Result<Unit> = runCatching {
        if (isRunning()) {
            return@runCatching
        }
        val bin = ensureBinary().getOrThrow()
        val pb = ProcessBuilder(bin.absolutePath, "run", "-c", configPath)
            .directory(context.filesDir)
            .redirectErrorStream(true)
        process = pb.start()
        Logs.add("[sing-box] started")
        val proc = process ?: return@runCatching
        ioExec.execute {
            proc.inputStream.bufferedReader().useLines { lines ->
                lines.forEach { Logs.add("[sing-box] $it") }
            }
        }
    }

    @Synchronized
    fun stop() {
        val p = process ?: return
        try {
            p.destroy()
            p.waitFor()
            Logs.add("[sing-box] stopped")
        } catch (e: Exception) {
            Logs.add("[sing-box] stop error: ${e.message}")
        } finally {
            process = null
        }
    }

    @Synchronized
    fun restart(configPath: String): Result<Unit> {
        stop()
        return start(configPath)
    }

    @Synchronized
    fun isRunning(): Boolean = process?.isAlive == true
}

