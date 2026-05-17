package com.sunlionet.agent

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class VpnLeakPolicyTest {
    @Test
    fun dnsServersAreTunnelLocal() {
        assertEquals("10.0.0.1", VpnLeakPolicy.IPV4_DNS)
        assertEquals("fd00:736c:6e::1", VpnLeakPolicy.IPV6_DNS)
        assertFalse(VpnLeakPolicy.IPV4_DNS.startsWith("1.1.1.1"))
        assertFalse(VpnLeakPolicy.IPV4_DNS.startsWith("8.8.8.8"))
    }

    @Test
    fun holdRequiresDesiredConnectionAndInterface() {
        assertTrue(VpnLeakPolicy.shouldHoldTunnel(desiredConnected = true, hasInterface = true))
        assertFalse(VpnLeakPolicy.shouldHoldTunnel(desiredConnected = false, hasInterface = true))
        assertFalse(VpnLeakPolicy.shouldHoldTunnel(desiredConnected = true, hasInterface = false))
    }
}
