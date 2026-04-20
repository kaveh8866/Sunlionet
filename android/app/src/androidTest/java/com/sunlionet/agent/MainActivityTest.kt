package com.sunlionet.agent

import android.content.Context
import java.io.File
import androidx.test.core.app.ApplicationProvider
import androidx.test.core.app.ActivityScenario
import androidx.test.espresso.Espresso.onView
import androidx.test.espresso.action.ViewActions.click
import androidx.test.espresso.assertion.ViewAssertions.matches
import androidx.test.espresso.matcher.ViewMatchers.isDisplayed
import androidx.test.espresso.matcher.ViewMatchers.withId
import androidx.test.espresso.matcher.ViewMatchers.withText
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class MainActivityTest {
    @Before
    fun resetState() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        SecureStore(context).setDesiredConnected(false)
        SecureStore(context).setAdvancedModeEnabled(false)
        StateRepository(context).save(UiState(status = "Disconnected", currentProfile = "-", lastAction = "-", lastError = "", lastErrorDetails = ""))
        runCatching {
            File(context.filesDir, "state/profiles.enc").delete()
        }
    }

    @Test
    fun mainScreenRenders() {
        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.textStatus)).check(matches(isDisplayed()))
            onView(withId(R.id.textStatusDetail)).check(matches(isDisplayed()))
            onView(withId(R.id.textConfigStatus)).check(matches(isDisplayed()))
            onView(withId(R.id.buttonToggle)).check(matches(isDisplayed()))
        }
    }

    @Test
    fun advancedPanelToggles() {
        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.buttonAdvanced)).perform(click())
            onView(withId(R.id.panelAdvanced)).check(matches(isDisplayed()))
        }
    }

    @Test
    fun connectCtaOpensImportOptionsWhenNoConfig() {
        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.buttonToggle)).perform(click())
            onView(withText(R.string.scan_qr)).check(matches(isDisplayed()))
        }
    }

    @Test
    fun connectedStateRendersDisconnectCta() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        SecureStore(context).setDesiredConnected(true)
        runCatching {
            val stateDir = File(context.filesDir, "state")
            stateDir.mkdirs()
            File(stateDir, "profiles.enc").writeBytes(byteArrayOf(1, 2, 3))
        }
        StateRepository(context).save(
            UiState(
                status = "Connected",
                currentProfile = "-",
                lastAction = "Connected",
                lastError = "",
                lastErrorDetails = "",
            ),
        )

        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.textStatus)).check(matches(withText(R.string.status_connected)))
            onView(withId(R.id.buttonToggle)).check(matches(withText(R.string.disconnect)))
        }
    }

    @Test
    fun errorStateRendersReadableMessage() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        SecureStore(context).setDesiredConnected(false)
        StateRepository(context).save(
            UiState(
                status = "Error",
                currentProfile = "-",
                lastAction = "Connection failed",
                lastError = context.getString(R.string.error_network_blocked),
                lastErrorDetails = "Timeout detected",
            ),
        )

        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.textStatus)).check(matches(withText(R.string.status_failed)))
            onView(withId(R.id.textStatusDetail)).check(matches(isDisplayed()))
        }
    }
}
