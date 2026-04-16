package com.shadownet.agent

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import android.os.Bundle
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
    private lateinit var textStatus: TextView
    private lateinit var textProfile: TextView
    private lateinit var textAction: TextView
    private lateinit var textLogs: TextView

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
        textStatus = findViewById(R.id.textStatus)
        textProfile = findViewById(R.id.textProfile)
        textAction = findViewById(R.id.textAction)
        textLogs = findViewById(R.id.textLogs)

        findViewById<Button>(R.id.buttonConnect).setOnClickListener { requestVpnAndConnect() }
        findViewById<Button>(R.id.buttonDisconnect).setOnClickListener { disconnect() }
        findViewById<Button>(R.id.buttonImport).setOnClickListener {
            importLauncher.launch(arrayOf("*/*"))
        }

        Logs.observe { text -> runOnUiThread { textLogs.text = text } }
        render(repo.load())

        lifecycleScope.launch {
            while (isActive) {
                val st = repo.load()
                render(st)
                delay(1000)
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
        startService(Intent(this, AgentService::class.java).apply { action = AgentService.ACTION_STOP })
        startService(Intent(this, ShadowNetVpnService::class.java).apply { action = ShadowNetVpnService.ACTION_STOP })
        repo.save(UiState(status = "Disconnected", currentProfile = "-", lastAction = "manual stop"))
        Logs.add("[ui] disconnect requested")
    }

    private fun render(st: UiState) {
        val status = if (st.lastError.isBlank()) st.status else "Error"
        textStatus.text = "Status: $status"
        textProfile.text = "Profile: ${st.currentProfile}"
        textAction.text = if (st.lastError.isBlank()) {
            "Last action: ${st.lastAction}"
        } else {
            "Error: ${st.lastError}"
        }
    }
}

