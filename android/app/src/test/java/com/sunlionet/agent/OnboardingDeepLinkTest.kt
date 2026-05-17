package com.sunlionet.agent

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class OnboardingDeepLinkTest {
    @Test
    fun normalizesFullAndShortForms() {
        val token = "AbC_123-def"
        assertEquals(
            "sunlionet://config/$token",
            OnboardingDeepLink.normalize(" sunlionet://config/$token\n"),
        )
        assertEquals(
            "sunlionet://config/$token",
            OnboardingDeepLink.normalize("SL1:$token"),
        )
    }

    @Test
    fun rejectsOversizeAndUnsafeText() {
        assertNull(OnboardingDeepLink.normalize("snb://v2:abc"))
        assertNull(OnboardingDeepLink.normalize("sunlionet://config/../../state"))
        assertNull(OnboardingDeepLink.normalize("sunlionet://config/${"a".repeat(301)}"))
    }
}
