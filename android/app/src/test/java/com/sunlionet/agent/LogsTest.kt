package com.sunlionet.agent

import org.junit.Assert.assertTrue
import org.junit.Test

class LogsTest {
    @Test
    fun addAndDump_shouldContainLatestEntry() {
        Logs.add("hello")
        assertTrue(Logs.dump().contains("hello"))
    }
}

