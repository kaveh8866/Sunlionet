package com.shadownet.agent

import android.app.Activity
import android.content.Intent
import android.graphics.Color
import android.net.VpnService
import android.os.Bundle
import android.view.View
import android.widget.Button
import android.widget.TextView
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.File

class MainActivity : AppCompatActivity() {
    private lateinit var repo: StateRepository
    private lateinit var secure: SecureStore
    private lateinit var statusDot: View
    private lateinit var textStatus: TextView
    private lateinit var textProfile: TextView
    private lateinit var textAction: TextView
    private lateinit var textError: TextView
    private lateinit var buttonToggle: Button

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            startRuntime()
        } else {
            Logs.add("[ui] vpn permission denied")
        }
    }

    private val importLauncher = registerForActivityResult(
        ActivityResultContracts.OpenDocument(),
    ) { uri ->
        if (uri == null) {
            return@registerForActivityResult
        }
        val out = File(cacheDir, "import.bundle")
        runCatching {
            contentResolver.openInputStream(uri).use { input ->
                requireNotNull(input) { "failed to open selected file" }
                out.outputStream().use { output -> input.copyTo(output) }
            }
            val intent = Intent(this, AgentService::class.java).apply {
                action = AgentService.ACTION_IMPORT
                putExtra(AgentService.EXTRA_BUNDLE_PATH, out.absolutePath)
            }
            startService(intent)
            Logs.add("[ui] import requested: ${out.absolutePath}")
        }.onFailure {
            Logs.add("[ui] import failed: ${it.message}")
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        repo = StateRepository(this)
        secure = SecureStore(this)
        statusDot = findViewById(R.id.statusDot)
        textStatus = findViewById(R.id.textStatus)
        textProfile = findViewById(R.id.textProfile)
        textAction = findViewById(R.id.textAction)
        textError = findViewById(R.id.textError)
        buttonToggle = findViewById(R.id.buttonToggle)

        buttonToggle.setOnClickListener {
            if (secure.isDesiredConnected()) {
                disconnect()
            } else {
                requestVpnAndConnect()
            }
        }
        findViewById<Button>(R.id.buttonImport).setOnClickListener {
            importLauncher.launch(arrayOf("*/*"))
        }

        render(repo.load())

        lifecycleScope.launch {
            while (isActive) {
                val st = repo.load()
                render(st)
                val d = if (st.status == "Connected") 1000L else 3000L
                delay(d)
            }
        }
    }

    private fun requestVpnAndConnect() {
        val intent = VpnService.prepare(this)
        if (intent != null) {
            vpnPermissionLauncher.launch(intent)
        } else {
            startRuntime()
        }
    }

    private fun startRuntime() {
        secure.setDesiredConnected(true)
        val vpnIntent = Intent(this, ShadowNetVpnService::class.java).apply {
            action = ShadowNetVpnService.ACTION_START
        }
        startForegroundService(vpnIntent)

        val agentIntent = Intent(this, AgentService::class.java).apply {
            action = AgentService.ACTION_START
        }
        startForegroundService(agentIntent)
        Logs.add("[ui] connect requested")
    }

    private fun disconnect() {
        secure.setDesiredConnected(false)
        startService(Intent(this, AgentService::class.java).apply { action = AgentService.ACTION_STOP })
        startService(Intent(this, ShadowNetVpnService::class.java).apply { action = ShadowNetVpnService.ACTION_STOP })
        repo.save(UiState(status = "Disconnected", currentProfile = "-", lastAction = "manual stop"))
        Logs.add("[ui] disconnect requested")
    }

    private fun render(st: UiState) {
        val showError = st.lastError.isNotBlank()
        val status = if (showError) "Error" else st.status
        val dotColor = if (showError) {
            Color.parseColor("#ef4444")
        } else if (st.status == "Connected") {
            Color.parseColor("#10b981")
        } else {
            Color.parseColor("#f59e0b")
        }

        statusDot.setBackgroundColor(dotColor)
        textStatus.text = "Status: $status"
        textProfile.text = "Profile: ${st.currentProfile}"
        textAction.text = "Last action: ${st.lastAction}"
        textError.text = "Last error: ${st.lastError.ifBlank { "-" }}"
        textError.visibility = if (showError) View.VISIBLE else View.GONE
        buttonToggle.text = if (secure.isDesiredConnected()) getString(R.string.disconnect) else getString(R.string.connect)
    }
}
