package com.shadownet.agent

import android.app.Activity
import android.content.Intent
import android.graphics.Color
import android.net.VpnService
import android.os.Bundle
import android.util.Base64
import android.view.View
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.Switch
import android.widget.TextView
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.appcompat.app.AlertDialog
import androidx.core.content.FileProvider
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.File

class MainActivity : AppCompatActivity() {
    private lateinit var repo: StateRepository
    private lateinit var secure: SecureStore
    private lateinit var diagnostics: DiagnosticsStore
    private lateinit var statusDot: View
    private lateinit var textAppLabel: TextView
    private lateinit var textVersion: TextView
    private lateinit var textSafety: TextView
    private lateinit var textHint: TextView
    private lateinit var textStatus: TextView
    private lateinit var textStatusDetail: TextView
    private lateinit var textProfile: TextView
    private lateinit var textAction: TextView
    private lateinit var textError: TextView
    private lateinit var textDebugPanel: TextView
    private lateinit var buttonToggle: Button
    private lateinit var sectionImport: LinearLayout
    private lateinit var buttonImport: Button
    private lateinit var buttonScanQr: Button
    private lateinit var buttonPasteLink: Button
    private lateinit var buttonAdvanced: Button
    private lateinit var panelAdvanced: LinearLayout
    private lateinit var switchDiagnostics: Switch
    private lateinit var buttonExportLogs: Button
    private lateinit var buttonReportIssue: Button

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
            requestImportBundle(out)
            Logs.add("[ui] import requested: ${out.absolutePath}")
        }.onFailure {
            Logs.add("[ui] import failed: ${it.message}")
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        RuntimeSignals.init(this)
        repo = StateRepository(this)
        secure = SecureStore(this)
        secure.ensureDefaultTrustAnchors()
        diagnostics = DiagnosticsStore(this)
        statusDot = findViewById(R.id.statusDot)
        textAppLabel = findViewById(R.id.textAppLabel)
        textVersion = findViewById(R.id.textVersion)
        textSafety = findViewById(R.id.textSafety)
        textHint = findViewById(R.id.textHint)
        textStatus = findViewById(R.id.textStatus)
        textStatusDetail = findViewById(R.id.textStatusDetail)
        textProfile = findViewById(R.id.textProfile)
        textAction = findViewById(R.id.textAction)
        textError = findViewById(R.id.textError)
        textDebugPanel = findViewById(R.id.textDebugPanel)
        buttonToggle = findViewById(R.id.buttonToggle)
        sectionImport = findViewById(R.id.sectionImport)
        buttonImport = findViewById(R.id.buttonImport)
        buttonScanQr = findViewById(R.id.buttonScanQr)
        buttonPasteLink = findViewById(R.id.buttonPasteLink)
        buttonAdvanced = findViewById(R.id.buttonAdvanced)
        panelAdvanced = findViewById(R.id.panelAdvanced)
        switchDiagnostics = findViewById(R.id.switchDiagnostics)
        buttonExportLogs = findViewById(R.id.buttonExportLogs)
        buttonReportIssue = findViewById(R.id.buttonReportIssue)

        val isTester = BuildConfig.TESTER_MODE
        textAppLabel.text = if (isTester) getString(R.string.app_name_test) else getString(R.string.app_name)
        textVersion.text = getString(R.string.version_label, BuildConfig.APP_VERSION_LABEL)
        textSafety.visibility = if (isTester) View.VISIBLE else View.GONE

        buttonToggle.setOnClickListener {
            val st = repo.load()
            val hasBundle = hasBundle()
            if (!hasBundle) {
                return@setOnClickListener
            }
            val desired = secure.isDesiredConnected()
            if (desired && st.status == "Connected") {
                disconnect()
                return@setOnClickListener
            }
            if (!desired) {
                requestVpnAndConnect()
            }
        }
        buttonImport.setOnClickListener {
            importLauncher.launch(arrayOf("*/*"))
        }
        buttonPasteLink.setOnClickListener { pasteLinkDialog() }
        buttonScanQr.setOnClickListener { launchQrScanner() }

        buttonAdvanced.setOnClickListener {
            secure.setAdvancedModeEnabled(!secure.isAdvancedModeEnabled())
            render(repo.load())
        }

        textError.setOnClickListener {
            val st = repo.load()
            if (st.lastError.isBlank() || st.lastErrorDetails.isBlank()) {
                return@setOnClickListener
            }
            AlertDialog.Builder(this)
                .setTitle(getString(R.string.details_title))
                .setMessage(st.lastErrorDetails)
                .setPositiveButton(getString(R.string.details_close), null)
                .show()
        }

        switchDiagnostics.isChecked = diagnostics.isDiagnosticsEnabled()
        switchDiagnostics.visibility = if (isTester) View.VISIBLE else View.GONE
        switchDiagnostics.setOnCheckedChangeListener { _, checked ->
            diagnostics.setDiagnosticsEnabled(checked)
            Logs.i("ui", "anonymous diagnostics=${if (checked) "on" else "off"}")
        }
        buttonExportLogs.visibility = if (isTester) View.VISIBLE else View.GONE
        buttonReportIssue.visibility = if (isTester) View.VISIBLE else View.GONE
        buttonExportLogs.setOnClickListener { exportLogs() }
        buttonReportIssue.setOnClickListener { reportIssue() }
        textDebugPanel.visibility = if (isTester) View.VISIBLE else View.GONE
        if (isTester) {
            Logs.observe { textDebugPanel.post { textDebugPanel.text = it } }
        }

        val loaded = repo.load()
        if (loaded.lastError.isBlank()) {
            val lastErr = diagnostics.lastErrorLabel()
            if (lastErr != "-") {
                repo.save(loaded.copy(lastError = lastErr))
            }
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
        repo.save(
            UiState(
                status = "Connecting",
                currentProfile = "-",
                lastAction = "Connecting…",
                lastError = "",
                lastErrorDetails = "",
            ),
        )
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

    private fun requestImportBundle(path: File) {
        val intent = Intent(this, AgentService::class.java).apply {
            action = AgentService.ACTION_IMPORT
            putExtra(AgentService.EXTRA_BUNDLE_PATH, path.absolutePath)
        }
        startService(intent)
    }

    private fun pasteLinkDialog() {
        val input = EditText(this).apply {
            hint = "snb://v2:…"
            setSingleLine(false)
            minLines = 3
        }
        AlertDialog.Builder(this)
            .setTitle(getString(R.string.paste_link))
            .setView(input)
            .setPositiveButton(getString(R.string.import_configuration)) { _, _ ->
                val text = input.text?.toString().orEmpty().trim()
                if (text.isBlank()) {
                    return@setPositiveButton
                }
                importFromText(text, "paste link")
            }
            .setNegativeButton(android.R.string.cancel, null)
            .show()
    }

    private val qrLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        if (result.resultCode != Activity.RESULT_OK) {
            return@registerForActivityResult
        }
        val text = result.data?.getStringExtra(QrScanActivity.EXTRA_QR_TEXT).orEmpty().trim()
        if (text.isBlank()) {
            return@registerForActivityResult
        }
        importFromText(text, "qr")
    }

    private fun launchQrScanner() {
        qrLauncher.launch(Intent(this, QrScanActivity::class.java))
    }

    private fun hasBundle(): Boolean {
        val f = File(filesDir, "state/profiles.enc")
        return f.exists() && f.length() > 0
    }

    private fun importFromText(text: String, source: String) {
        val normalized = text.trim()
        if (!normalized.startsWith("snb://v2:")) {
            AlertDialog.Builder(this)
                .setTitle(getString(R.string.import_configuration))
                .setMessage(getString(R.string.error_config_invalid))
                .setPositiveButton(getString(R.string.details_close), null)
                .show()
            Logs.add("[ui] import failed: invalid uri source=$source")
            return
        }

        val out = File(cacheDir, "import.bundle")
        runCatching {
            val raw = normalized.removePrefix("snb://v2:")
            val padded = when (raw.length % 4) {
                2 -> "$raw=="
                3 -> "$raw="
                else -> raw
            }
            val bytes = Base64.decode(padded, Base64.URL_SAFE or Base64.NO_WRAP)
            out.writeBytes(bytes)
            requestImportBundle(out)
            Logs.add("[ui] import requested: $source")
        }.onFailure {
            AlertDialog.Builder(this)
                .setTitle(getString(R.string.import_configuration))
                .setMessage(getString(R.string.error_config_invalid))
                .setPositiveButton(getString(R.string.details_close), null)
                .show()
            Logs.add("[ui] import failed source=$source err=${it.message}")
        }
    }

    private fun exportLogs() {
        runCatching {
            val file = diagnostics.exportLogsJson(Logs.dump())
            val uri = FileProvider.getUriForFile(this, "${packageName}.provider", file)
            val send = Intent(Intent.ACTION_SEND).apply {
                type = "application/json"
                putExtra(Intent.EXTRA_STREAM, uri)
                putExtra(Intent.EXTRA_SUBJECT, "ShadowNet logs (${BuildConfig.APP_VERSION_LABEL})")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            }
            startActivity(Intent.createChooser(send, getString(R.string.export_logs)))
            Logs.i("ui", "logs exported")
        }.onFailure {
            Logs.e("ui", "export logs failed: ${it.message}")
        }
    }

    private fun reportIssue() {
        runCatching {
            val file = diagnostics.exportLogsJson(Logs.dump())
            val uri = FileProvider.getUriForFile(this, "${packageName}.provider", file)
            val body = buildString {
                appendLine("Version: ${BuildConfig.APP_VERSION_LABEL}")
                appendLine("Tester mode: ${BuildConfig.TESTER_MODE}")
                appendLine("Describe what happened (without sharing personal data):")
                appendLine("- Network type (Wi-Fi/mobile)")
                appendLine("- Failure category")
                appendLine("- Steps to reproduce")
            }
            val intent = Intent(Intent.ACTION_SEND).apply {
                type = "message/rfc822"
                putExtra(Intent.EXTRA_SUBJECT, "ShadowNet Test Build Issue")
                putExtra(Intent.EXTRA_TEXT, body)
                putExtra(Intent.EXTRA_STREAM, uri)
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            }
            startActivity(Intent.createChooser(intent, getString(R.string.report_issue)))
            Logs.i("ui", "report issue opened")
        }.onFailure {
            Logs.e("ui", "report issue failed: ${it.message}")
        }
    }

    private fun render(st: UiState) {
        val hasBundle = hasBundle()
        val desired = secure.isDesiredConnected()
        val status = when {
            st.lastError.isNotBlank() -> "Failed"
            desired && st.status != "Connected" -> "Connecting"
            st.status == "Connected" -> "Connected"
            else -> "Disconnected"
        }

        val dotColor = when (status) {
            "Connected" -> Color.parseColor("#10b981")
            "Connecting" -> Color.parseColor("#f59e0b")
            "Failed" -> Color.parseColor("#ef4444")
            else -> Color.parseColor("#9ca3af")
        }

        statusDot.setBackgroundColor(dotColor)
        textStatus.text = when (status) {
            "Connected" -> getString(R.string.status_connected)
            "Connecting" -> getString(R.string.status_connecting)
            "Failed" -> getString(R.string.status_failed)
            else -> getString(R.string.status_disconnected)
        }
        textStatusDetail.text = when (status) {
            "Connected" -> getString(R.string.status_detail_connected)
            "Connecting" -> getString(R.string.status_detail_connecting)
            "Failed" -> getString(R.string.status_detail_failed)
            else -> getString(R.string.status_detail_disconnected)
        }

        val showError = st.lastError.isNotBlank()
        textError.text = st.lastError
        textError.visibility = if (showError) View.VISIBLE else View.GONE

        sectionImport.visibility = if (hasBundle) View.GONE else View.VISIBLE
        textHint.text = if (hasBundle) getString(R.string.hint_ready) else getString(R.string.hint_welcome)
        if (status == "Failed") {
            textHint.text = getString(R.string.hint_retry)
        }

        val advanced = secure.isAdvancedModeEnabled()
        panelAdvanced.visibility = if (advanced) View.VISIBLE else View.GONE
        buttonAdvanced.text = if (advanced) getString(R.string.hide_advanced) else getString(R.string.show_advanced)

        textProfile.text = "Profile: ${st.currentProfile}"
        textAction.text = "Last action: ${st.lastAction}"

        buttonToggle.text = when {
            !hasBundle -> getString(R.string.connect)
            desired && st.status == "Connected" -> getString(R.string.disconnect)
            desired -> getString(R.string.status_connecting)
            else -> getString(R.string.connect)
        }
        buttonToggle.isEnabled = hasBundle && !(desired && st.status != "Connected")
    }
}
