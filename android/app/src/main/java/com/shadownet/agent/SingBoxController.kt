package com.shadownet.agent

import android.content.Context
import java.io.BufferedReader
import java.io.File
import java.io.FileNotFoundException
import java.security.MessageDigest
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.TimeUnit
import java.util.regex.Pattern

class SingBoxController(private val context: Context) {
    private var process: Process? = null
    private var desiredConfigPath: String? = null
    private var restartAttempts = 0
    private val ioExec = Executors.newCachedThreadPool()
    private val supervisor: ScheduledExecutorService = Executors.newSingleThreadScheduledExecutor()
    private val ipPattern = Pattern.compile("\\b(?:\\d{1,3}\\.){3}\\d{1,3}\\b")

    private fun binaryFile(): File = File(context.filesDir, "bin/sing-box")

    private data class AbiInfo(
        val abi: String,
        val assetName: String,
        val expectedSha256: String,
    )

    private fun abiInfo(): AbiInfo {
        val abi = android.os.Build.SUPPORTED_ABIS.firstOrNull().orEmpty()
        return when {
            abi.contains("x86") -> AbiInfo(
                abi = abi,
                assetName = "",
                expectedSha256 = "",
            )
            abi.contains("arm64") -> AbiInfo(
                abi = abi,
                assetName = "bin/sing-box-arm64",
                expectedSha256 = BuildConfig.SING_BOX_SHA256_ARM64,
            )
            abi.contains("armeabi") -> AbiInfo(
                abi = abi,
                assetName = "bin/sing-box-armv7",
                expectedSha256 = BuildConfig.SING_BOX_SHA256_ARMV7,
            )
            else -> AbiInfo(
                abi = abi,
                assetName = "bin/sing-box-arm64",
                expectedSha256 = BuildConfig.SING_BOX_SHA256_ARM64,
            )
        }
    }

    fun ensureBinary(): Result<File> = runCatching {
        val abiInfo = abiInfo()
        if (abiInfo.abi.contains("x86")) {
            throw IllegalStateException(
                "sing-box is not packaged for ABI=${abiInfo.abi}. This is expected on x86/x86_64 emulators. Use an ARM64 device/emulator or add x86_64 assets.",
            )
        }

        val out = binaryFile()
        if (!out.exists()) {
            out.parentFile?.mkdirs()
            try {
                context.assets.open(abiInfo.assetName).use { input ->
                    out.outputStream().use { output -> input.copyTo(output) }
                }
            } catch (e: FileNotFoundException) {
                throw IllegalStateException("sing-box binary missing in assets: ${abiInfo.assetName} (ABI=${abiInfo.abi})")
            }
        }
        if (!out.setExecutable(true)) {
            throw IllegalStateException("failed to mark sing-box executable")
        }

        val expected = abiInfo.expectedSha256.trim()
        if (expected.isBlank()) {
            if (BuildConfig.DEBUG) {
                Logs.w("sing-box", "checksum unavailable for ABI=${abiInfo.abi} (debug build); skipping verification")
                return@runCatching out
            }
            throw IllegalStateException("expected sing-box sha256 missing for ABI=${abiInfo.abi}")
        }

        val actual = sha256(out)
        if (!actual.equals(expected, ignoreCase = true)) {
            throw IllegalStateException("sing-box checksum mismatch: expected=$expected actual=$actual")
        }

        out
    }

    private fun sha256(file: File): String {
        val md = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buf = ByteArray(32 * 1024)
            while (true) {
                val n = input.read(buf)
                if (n <= 0) {
                    break
                }
                md.update(buf, 0, n)
            }
        }
        val sum = md.digest()
        return sum.joinToString("") { b -> "%02x".format(b) }
    }

    @Synchronized
    fun start(configPath: String): Result<Unit> = runCatching {
        if (isRunning()) {
            return@runCatching
        }
        val bin = ensureBinary().getOrThrow()
        if (desiredConfigPath == null || desiredConfigPath != configPath) {
            restartAttempts = 0
        }
        desiredConfigPath = configPath
        validateConfig(bin, configPath)
        val pb = ProcessBuilder(bin.absolutePath, "run", "-c", configPath)
            .directory(context.filesDir)
            .redirectErrorStream(false)
        process = pb.start()
        val pid = pidOf(process)
        Logs.i("sing-box", "started pid=$pid")
        val proc = process ?: return@runCatching
        ioExec.execute { pump(proc.inputStream.bufferedReader(), "stdout") }
        ioExec.execute { pump(proc.errorStream.bufferedReader(), "stderr") }
        ioExec.execute { waitForExit(proc) }
    }

    @Synchronized
    fun stop() {
        desiredConfigPath = null
        val p = process ?: return
        try {
            p.destroy()
            p.waitFor()
            Logs.i("sing-box", "stopped")
        } catch (e: Exception) {
            Logs.e("sing-box", "stop error: ${e.message}")
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

    private fun validateConfig(bin: File, configPath: String) {
        val pb = ProcessBuilder(bin.absolutePath, "check", "-c", configPath)
            .directory(context.filesDir)
            .redirectErrorStream(true)
        val p = pb.start()
        val out = runCatching { p.inputStream.bufferedReader().readText() }.getOrDefault("")
        val code = runCatching { p.waitFor() }.getOrDefault(-1)
        if (code != 0) {
            throw IllegalStateException("sing-box failed: config invalid")
        }
        if (out.isNotBlank()) {
            Logs.i("sing-box", "check ok: ${sanitizeLine(out)}")
        } else {
            Logs.i("sing-box", "check ok")
        }
    }

    private fun pump(reader: BufferedReader, stream: String) {
        runCatching {
            reader.useLines { lines ->
                lines.forEach { line ->
                    Logs.i("sing-box", "$stream ${sanitizeLine(line)}")
                }
            }
        }
    }

    private fun waitForExit(proc: Process) {
        val code = runCatching { proc.waitFor() }.getOrDefault(-1)
        val shouldRestart: Boolean = synchronized(this) {
            if (process !== proc) {
                false
            } else {
                process = null
                desiredConfigPath != null
            }
        }
        Logs.w("sing-box", "exited code=$code desired=$shouldRestart")
        if (!shouldRestart) {
            return
        }
        scheduleRestart()
    }

    private fun scheduleRestart() {
        val cfg: String? = synchronized(this) {
            val path = desiredConfigPath
            if (path.isNullOrBlank()) {
                null
            } else if (restartAttempts >= 3) {
                Logs.e("sing-box", "restart limit reached")
                null
            } else {
                restartAttempts++
                path
            }
        }
        if (cfg.isNullOrBlank()) return
        val backoffMs = when (restartAttempts) {
            1 -> 1000L
            2 -> 2000L
            else -> 4000L
        }
        supervisor.schedule(
            {
                start(cfg).onFailure { Logs.e("sing-box", "restart failed: ${it.message}") }
            },
            backoffMs,
            TimeUnit.MILLISECONDS,
        )
    }

    private fun pidOf(proc: Process?): Long {
        if (proc == null) {
            return -1
        }
        return runCatching {
            val m = proc.javaClass.getMethod("pid")
            (m.invoke(proc) as? Long) ?: -1L
        }.getOrDefault(-1L)
    }

    private fun sanitizeLine(input: String): String {
        var out = input.trim()
        out = ipPattern.matcher(out).replaceAll("x.x.x.x")
        if (out.length > 200) {
            out = out.take(200) + "…"
        }
        return out
    }
}
