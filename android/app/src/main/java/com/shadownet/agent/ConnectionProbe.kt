package com.shadownet.agent

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withTimeout
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL
import kotlin.coroutines.resume

object ConnectionProbe {
    data class ResultInfo(
        val status: String,
        val reason: String,
        val httpStatus: Int? = null,
        val error: String? = null,
    )

    suspend fun probeHttpViaVpn(context: Context, targetUrl: String, timeoutMs: Long): ResultInfo {
        return try {
            val net = awaitVpnNetwork(context, timeoutMs = timeoutMs.coerceAtLeast(1000))
            val code = withTimeout(timeoutMs.coerceAtLeast(1000)) {
                val url = URL(targetUrl)
                val conn = (net.openConnection(url) as HttpURLConnection).apply {
                    connectTimeout = timeoutMs.toInt().coerceAtMost(15_000)
                    readTimeout = timeoutMs.toInt().coerceAtMost(15_000)
                    instanceFollowRedirects = true
                    requestMethod = "GET"
                }
                conn.inputStream.use { input ->
                    val buf = ByteArray(64 * 1024)
                    input.read(buf)
                }
                conn.responseCode
            }
            if (code in 200..399) {
                ResultInfo(status = "ok", reason = "OK", httpStatus = code)
            } else {
                ResultInfo(status = "failed", reason = "HTTP_${code}", httpStatus = code, error = "unexpected status")
            }
        } catch (e: Exception) {
            ResultInfo(status = "failed", reason = classify(e), error = e.message ?: e.javaClass.simpleName)
        }
    }

    private suspend fun awaitVpnNetwork(context: Context, timeoutMs: Long): Network {
        val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        return withTimeout(timeoutMs) {
            suspendCancellableCoroutine { cont ->
                val req = NetworkRequest.Builder()
                    .addTransportType(NetworkCapabilities.TRANSPORT_VPN)
                    .build()
                val cb = object : ConnectivityManager.NetworkCallback() {
                    override fun onAvailable(network: Network) {
                        runCatching { cm.unregisterNetworkCallback(this) }
                        cont.resume(network)
                    }
                }
                cm.registerNetworkCallback(req, cb)
                cont.invokeOnCancellation {
                    runCatching { cm.unregisterNetworkCallback(cb) }
                }
            }
        }
    }

    private fun classify(e: Exception): String {
        val msg = (e.message ?: "").lowercase()
        return when {
            e is java.net.UnknownHostException -> "DNS_BLOCKED"
            msg.contains("no such host") -> "DNS_BLOCKED"
            msg.contains("tls") && (msg.contains("handshake") || msg.contains("certificate")) -> "TLS_BLOCKED"
            msg.contains("connection reset") || msg.contains("broken pipe") -> "TCP_RESET"
            msg.contains("network is unreachable") || msg.contains("no route to host") -> "NO_ROUTE"
            msg.contains("timeout") -> "TIMEOUT"
            e is java.net.SocketTimeoutException -> "TIMEOUT"
            e is IOException && msg.contains("refused") -> "TCP_RESET"
            else -> "UNKNOWN"
        }
    }
}
