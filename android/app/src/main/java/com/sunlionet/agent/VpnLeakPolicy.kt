package com.sunlionet.agent

object VpnLeakPolicy {
    const val IPV4_ADDRESS = "10.0.0.2"
    const val IPV4_PREFIX = 32
    const val IPV4_DNS = "10.0.0.1"
    const val IPV6_ADDRESS = "fd00:736c:6e::2"
    const val IPV6_PREFIX = 128
    const val IPV6_DNS = "fd00:736c:6e::1"
    const val MTU = 1400

    fun shouldHoldTunnel(desiredConnected: Boolean, hasInterface: Boolean): Boolean {
        return desiredConnected && hasInterface
    }
}
