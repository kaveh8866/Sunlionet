package com.sunlionet.agent

import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.SocketTimeoutException
import java.net.UnknownHostException

class LogsTest {
    @Test
    fun addAndDump_shouldContainLatestEntry() {
        Logs.add("hello")
        assertTrue(Logs.dump().contains("hello"))
    }

    @Test
    fun add_shouldRedactSensitiveMarkers() {
        val sixtyFourHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
        Logs.add("ip=1.2.3.4 url=https://example.com/path secret=age-secret-key-abcdef token=$sixtyFourHex")
        val dumped = Logs.dump()
        assertTrue(dumped.contains("x.x.x.x"))
        assertTrue(dumped.contains("[url-redacted]"))
        assertTrue(dumped.contains("[redacted]"))
        assertTrue(!dumped.contains("1.2.3.4"))
        assertTrue(!dumped.contains("https://example.com"))
        assertTrue(!dumped.contains(sixtyFourHex))
    }

    @Test
    fun bridgeStatusParser_shouldMapRunningTrueToConnected() {
        val raw = """{"running":true,"current_profile":"p1","last_action":"connect","last_error":""}"""
        val state = StateRepository.parseBridgeStatus(raw)
        assertTrue(state.status == "Connected")
        assertTrue(state.currentProfile == "p1")
        assertTrue(state.lastAction == "connect")
    }

    @Test
    fun bridgeStatusParser_shouldFailClosedOnInvalidJson() {
        val state = StateRepository.parseBridgeStatus("{not-json")
        assertTrue(state.status == "Error")
        assertTrue(state.lastError.isNotBlank())
    }

    @Test
    fun probeClassifier_shouldDetectDnsTimeoutAndUnknown() {
        assertTrue(ConnectionProbe.classifyException(UnknownHostException("no such host")).startsWith("DNS"))
        assertTrue(ConnectionProbe.classifyException(SocketTimeoutException("timeout")).startsWith("TIMEOUT"))
        assertTrue(ConnectionProbe.classifyException(IllegalStateException("something else")) == "UNKNOWN")
    }
}
