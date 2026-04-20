package com.shadownet.agent

import android.Manifest
import android.app.Activity
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.util.Base64
import android.view.View
import android.text.method.ScrollingMovementMethod
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.Switch
import android.widget.TextView
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.appcompat.app.AlertDialog
import androidx.core.content.FileProvider
import androidx.core.content.ContextCompat
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
    private lateinit var textConfigStatus: TextView
    private lateinit var textProfile: TextView
    private lateinit var textAction: TextView
    private lateinit var textError: TextView
    private lateinit var textDebugPanel: TextView
    private lateinit var progressConnecting: ProgressBar
    private lateinit var buttonToggle: Button
    private lateinit var sectionImport: LinearLayout
    private lateinit var buttonImport: Button
    private lateinit var buttonScanQr: Button
    private lateinit var buttonPasteLink: Button
    private lateinit var buttonAdvanced: Button
    private lateinit var panelAdvanced: LinearLayout
    private lateinit var switchDiagnostics: Switch
    private lateinit var buttonPhase4Tools: Button
    private lateinit var buttonExportLogs: Button
    private lateinit var buttonReportIssue: Button

    private var pendingAfterNotificationPermission: (() -> Unit)? = null

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            startRuntime()
        } else {
            Logs.add("[ui] vpn permission denied")
            secure.setDesiredConnected(false)
            repo.save(
                UiState(
                    status = "Error",
                    currentProfile = "-",
                    lastAction = "Permission required",
                    lastError = getString(R.string.error_vpn_permission_denied),
                    lastErrorDetails = "",
                ),
            )
        }
    }

    private val notificationsPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { _ ->
        val next = pendingAfterNotificationPermission ?: return@registerForActivityResult
        pendingAfterNotificationPermission = null
        next()
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
        textConfigStatus = findViewById(R.id.textConfigStatus)
        textProfile = findViewById(R.id.textProfile)
        textAction = findViewById(R.id.textAction)
        textError = findViewById(R.id.textError)
        textDebugPanel = findViewById(R.id.textDebugPanel)
        progressConnecting = findViewById(R.id.progressConnecting)
        buttonToggle = findViewById(R.id.buttonToggle)
        sectionImport = findViewById(R.id.sectionImport)
        buttonImport = findViewById(R.id.buttonImport)
        buttonScanQr = findViewById(R.id.buttonScanQr)
        buttonPasteLink = findViewById(R.id.buttonPasteLink)
        buttonAdvanced = findViewById(R.id.buttonAdvanced)
        panelAdvanced = findViewById(R.id.panelAdvanced)
        switchDiagnostics = findViewById(R.id.switchDiagnostics)
        buttonPhase4Tools = findViewById(R.id.buttonPhase4Tools)
        buttonExportLogs = findViewById(R.id.buttonExportLogs)
        buttonReportIssue = findViewById(R.id.buttonReportIssue)

        val isTester = BuildConfig.TESTER_MODE
        textAppLabel.text = if (isTester) getString(R.string.app_name_test) else getString(R.string.app_name)
        textVersion.text = getString(R.string.version_label, BuildConfig.APP_VERSION_LABEL)
        textSafety.visibility = if (isTester) View.VISIBLE else View.GONE

        buttonToggle.setOnClickListener {
            val hasBundle = hasBundle()
            val desired = secure.isDesiredConnected()
            if (desired) {
                disconnect()
                return@setOnClickListener
            }
            if (!hasBundle) {
                if (canDevConnectWithoutBundle()) {
                    showImportOptionsDialog(allowDevConnect = true)
                } else {
                    showImportOptionsDialog(allowDevConnect = false)
                }
                return@setOnClickListener
            }
            ensureNotificationsPermissionThen { requestVpnAndConnect() }
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
        buttonPhase4Tools.visibility = if (isTester) View.VISIBLE else View.GONE
        buttonPhase4Tools.setOnClickListener { showPhase4ToolsDialog() }
        buttonExportLogs.visibility = if (isTester) View.VISIBLE else View.GONE
        buttonReportIssue.visibility = if (isTester) View.VISIBLE else View.GONE
        buttonExportLogs.setOnClickListener { exportLogs() }
        buttonReportIssue.setOnClickListener { reportIssue() }
        textDebugPanel.visibility = if (isTester) View.VISIBLE else View.GONE
        if (isTester) {
            textDebugPanel.movementMethod = ScrollingMovementMethod()
            Logs.observe { textDebugPanel.post { textDebugPanel.text = it } }
        }

        if (!hasBundle()) {
            secure.setDesiredConnected(false)
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

    private fun ensureNotificationsPermissionThen(action: () -> Unit) {
        if (Build.VERSION.SDK_INT < 33) {
            action()
            return
        }
        val granted =
            ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        if (granted) {
            action()
            return
        }
        pendingAfterNotificationPermission = action
        notificationsPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
    }

    private fun canDevConnectWithoutBundle(): Boolean {
        if (!BuildConfig.DEBUG) {
            return false
        }
        val fp = android.os.Build.FINGERPRINT.orEmpty()
        val model = android.os.Build.MODEL.orEmpty()
        val brand = android.os.Build.BRAND.orEmpty()
        val device = android.os.Build.DEVICE.orEmpty()
        val product = android.os.Build.PRODUCT.orEmpty()
        val hardware = android.os.Build.HARDWARE.orEmpty()
        return fp.contains("generic", ignoreCase = true) ||
            fp.contains("emulator", ignoreCase = true) ||
            fp.contains("sdk_gphone", ignoreCase = true) ||
            model.contains("Emulator", ignoreCase = true) ||
            model.contains("Android SDK built for", ignoreCase = true) ||
            brand.contains("generic", ignoreCase = true) ||
            device.contains("generic", ignoreCase = true) ||
            product.contains("sdk_gphone", ignoreCase = true) ||
            hardware.contains("ranchu", ignoreCase = true) ||
            hardware.contains("goldfish", ignoreCase = true)
    }

    private fun showImportFirstDialog() {
        AlertDialog.Builder(this)
            .setTitle(getString(R.string.import_configuration))
            .setMessage(getString(R.string.error_config_missing))
            .setPositiveButton(getString(R.string.details_close), null)
            .show()
    }

    private fun showImportOptionsDialog(allowDevConnect: Boolean) {
        val items = buildList {
            add(getString(R.string.import_configuration))
            add(getString(R.string.scan_qr))
            add(getString(R.string.paste_link))
            if (allowDevConnect) {
                add(getString(R.string.connect_dev))
            }
        }.toTypedArray()
        AlertDialog.Builder(this)
            .setTitle(getString(R.string.import_configuration))
            .setItems(items) { _, which ->
                val selected = items[which]
                when (selected) {
                    getString(R.string.import_configuration) -> importLauncher.launch(arrayOf("*/*"))
                    getString(R.string.scan_qr) -> launchQrScanner()
                    getString(R.string.paste_link) -> pasteLinkDialog()
                    getString(R.string.connect_dev) -> ensureNotificationsPermissionThen { requestVpnAndConnect() }
                }
            }
            .setNegativeButton(android.R.string.cancel, null)
            .show()
    }

    private fun showDevConnectDialog() {
        val msg = buildString {
            appendLine(getString(R.string.error_config_missing))
            appendLine()
            append("Dev/emulator mode: continue to exercise the VPN permission + service startup flow. No real connection will be established.")
        }
        AlertDialog.Builder(this)
            .setTitle(getString(R.string.connect))
            .setMessage(msg)
            .setPositiveButton(getString(R.string.connect)) { _, _ ->
                ensureNotificationsPermissionThen { requestVpnAndConnect() }
            }
            .setNegativeButton(android.R.string.cancel, null)
            .show()
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
                putExtra(Intent.EXTRA_SUBJECT, "SunLionet logs (${BuildConfig.APP_VERSION_LABEL})")
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
                putExtra(Intent.EXTRA_SUBJECT, "SunLionet Test Build Issue")
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
        val allowDevConnect = !hasBundle && canDevConnectWithoutBundle()
        val desired = secure.isDesiredConnected()
        val isConfigMissing = !hasBundle && st.lastError == getString(R.string.error_config_missing)
        val status = when {
            isConfigMissing -> "Disconnected"
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
            "Failed" -> {
                if (allowDevConnect) {
                    "Dev/emulator mode: no configuration imported. You can still exercise the VPN permission flow."
                } else {
                    getString(R.string.status_detail_failed)
                }
            }
            else -> getString(R.string.status_detail_disconnected)
        }

        progressConnecting.visibility = if (status == "Connecting") View.VISIBLE else View.GONE

        textConfigStatus.text = if (hasBundle) getString(R.string.config_ready) else getString(R.string.config_required)
        textConfigStatus.setTextColor(
            if (hasBundle) {
                Color.parseColor("#10b981")
            } else {
                Color.parseColor("#f59e0b")
            },
        )

        val showError = st.lastError.isNotBlank() && !isConfigMissing
        textError.text = st.lastError
        textError.contentDescription =
            if (st.lastErrorDetails.isNotBlank()) {
                "${st.lastError}. ${getString(R.string.details_title)}"
            } else {
                st.lastError
            }
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
            desired -> getString(R.string.disconnect)
            !hasBundle && allowDevConnect -> getString(R.string.connect_dev)
            else -> getString(R.string.connect)
        }
        buttonToggle.isEnabled = true
    }

    private fun showPhase4ToolsDialog() {
        fun showText(title: String, message: String) {
            AlertDialog.Builder(this)
                .setTitle(title)
                .setMessage(message)
                .setPositiveButton(getString(R.string.details_close), null)
                .show()
        }

        fun prompt(
            title: String,
            hint: String,
            initial: String = "",
            multiline: Boolean = false,
            onOk: (String) -> Unit,
        ) {
            val input = EditText(this).apply {
                setText(initial)
                setHint(hint)
                if (multiline) {
                    minLines = 3
                }
            }
            AlertDialog.Builder(this)
                .setTitle(title)
                .setView(input)
                .setPositiveButton("OK") { _, _ -> onOk(input.text?.toString().orEmpty()) }
                .setNegativeButton("Cancel", null)
                .show()
        }

        val actions = arrayOf(
            "Identity: Create persona",
            "Identity: List personas",
            "Identity: Create contact offer",
            "Chat: Add contact from offer",
            "Chat: List chats",
            "Chat: View messages",
            "Chat: Sync inbox",
            "Chat: Send 1:1 message",
            "Chat: Create group",
            "Chat: Invite to group",
            "Chat: Join group",
            "Chat: Set group role",
            "Chat: Remove from group",
            "Chat: Send group message",
            "Community: List",
            "Community: Create",
            "Community: Create invite",
            "Community: Create join request",
            "Community: Approve join",
            "Community: Apply join",
            "Community: Create room",
            "Community: Send post",
            "Device: Create Join Request",
            "Device: Approve Join Request",
            "Device: Apply Join Package",
        )

        AlertDialog.Builder(this)
            .setTitle("Phase 4 Tools")
            .setItems(actions) { _, which ->
                when (which) {
                    0 -> {
                        Bridge.createPersona(this).onSuccess {
                            showText("Persona created", it)
                        }.onFailure {
                            showText("Error", it.message ?: "failed")
                        }
                    }
                    1 -> {
                        Bridge.listPersonas(this).onSuccess {
                            showText("Personas", it)
                        }.onFailure {
                            showText("Error", it.message ?: "failed")
                        }
                    }
                    2 -> {
                        prompt("Persona ID", "persona id") { personaID ->
                            prompt("TTL (seconds)", "e.g. 600", initial = "600") { ttl ->
                                val ttlSec = ttl.trim().toIntOrNull() ?: 600
                                Bridge.createContactOffer(this, personaID.trim(), ttlSec).onSuccess {
                                    showText("Contact offer", it)
                                }.onFailure {
                                    showText("Error", it.message ?: "failed")
                                }
                            }
                        }
                    }
                    3 -> {
                        prompt("Alias", "e.g. Alice", initial = "Alice") { alias ->
                            prompt("Offer token", "sn4:...", multiline = true) { offer ->
                                Bridge.chatAddContactFromOffer(this, alias.trim(), offer.trim()).onSuccess {
                                    showText("Contact added", it)
                                }.onFailure {
                                    showText("Error", it.message ?: "failed")
                                }
                            }
                        }
                    }
                    4 -> {
                        Bridge.chatList(this).onSuccess {
                            showText("Chats", it)
                        }.onFailure {
                            showText("Error", it.message ?: "failed")
                        }
                    }
                    5 -> {
                        prompt("Chat ID", "e.g. d:... or g:...") { chatID ->
                            Bridge.chatMessages(this, chatID.trim()).onSuccess {
                                showText("Messages", it)
                            }.onFailure {
                                showText("Error", it.message ?: "failed")
                            }
                        }
                    }
                    6 -> {
                        prompt("Relay URL", "https://relay.example.com") { relayURL ->
                            prompt("Persona ID", "persona id") { personaID ->
                                Bridge.chatSyncOnce(
                                    relayURL = relayURL.trim(),
                                    context = this,
                                    personaID = personaID.trim(),
                                ).onSuccess {
                                    showText("Synced", it)
                                }.onFailure {
                                    showText("Error", it.message ?: "failed")
                                }
                            }
                        }
                    }
                    7 -> {
                        prompt("Relay URL", "https://relay.example.com") { relayURL ->
                            prompt("Persona ID", "persona id") { personaID ->
                                prompt("Contact ID", "contact id") { contactID ->
                                    prompt("Message", "text", multiline = true) { text ->
                                        Bridge.chatSendText(
                                            relayURL = relayURL.trim(),
                                            context = this,
                                            personaID = personaID.trim(),
                                            contactID = contactID.trim(),
                                            text = text,
                                        ).onSuccess {
                                            showText("Sent", it)
                                        }.onFailure {
                                            showText("Error", it.message ?: "failed")
                                        }
                                    }
                                }
                            }
                        }
                    }
                    8 -> {
                        prompt("Title", "group title", initial = "Group") { title ->
                            prompt("Member IDs (CSV)", "contact_id1,contact_id2") { memberIDs ->
                                Bridge.chatCreateGroup(this, title.trim(), memberIDs.trim()).onSuccess {
                                    showText("Group created", it)
                                }.onFailure {
                                    showText("Error", it.message ?: "failed")
                                }
                            }
                        }
                    }
                    9 -> {
                        prompt("Relay URL", "https://relay.example.com") { relayURL ->
                            prompt("Persona ID", "persona id") { personaID ->
                                prompt("Group ID", "g:...") { groupID ->
                                    prompt("Invitee Contact ID", "contact id") { inviteeContactID ->
                                        Bridge.chatInviteToGroup(
                                            relayURL = relayURL.trim(),
                                            context = this,
                                            personaID = personaID.trim(),
                                            groupID = groupID.trim(),
                                            inviteeContactID = inviteeContactID.trim(),
                                        ).onSuccess {
                                            showText("Invited", it)
                                        }.onFailure {
                                            showText("Error", it.message ?: "failed")
                                        }
                                    }
                                }
                            }
                        }
                    }
                    10 -> {
                        prompt("Relay URL", "https://relay.example.com") { relayURL ->
                            prompt("Persona ID", "persona id") { personaID ->
                                prompt("Group ID", "g:...") { groupID ->
                                    Bridge.chatJoinGroup(
                                        relayURL = relayURL.trim(),
                                        context = this,
                                        personaID = personaID.trim(),
                                        groupID = groupID.trim(),
                                    ).onSuccess {
                                        showText("Join sent", it)
                                    }.onFailure {
                                        showText("Error", it.message ?: "failed")
                                    }
                                }
                            }
                        }
                    }
                    11 -> {
                        prompt("Relay URL", "https://relay.example.com") { relayURL ->
                            prompt("Persona ID", "persona id") { personaID ->
                                prompt("Group ID", "g:...") { groupID ->
                                    prompt("Subject Contact ID", "contact id") { subjectContactID ->
                                        prompt("Role", "owner|moderator|member", initial = "member") { role ->
                                            Bridge.chatSetGroupRole(
                                                relayURL = relayURL.trim(),
                                                context = this,
                                                personaID = personaID.trim(),
                                                groupID = groupID.trim(),
                                                subjectContactID = subjectContactID.trim(),
                                                role = role.trim(),
                                            ).onSuccess {
                                                showText("Role updated", it)
                                            }.onFailure {
                                                showText("Error", it.message ?: "failed")
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                    12 -> {
                        prompt("Relay URL", "https://relay.example.com") { relayURL ->
                            prompt("Persona ID", "persona id") { personaID ->
                                prompt("Group ID", "g:...") { groupID ->
                                    prompt("Subject Contact ID", "contact id") { subjectContactID ->
                                        Bridge.chatRemoveFromGroup(
                                            relayURL = relayURL.trim(),
                                            context = this,
                                            personaID = personaID.trim(),
                                            groupID = groupID.trim(),
                                            subjectContactID = subjectContactID.trim(),
                                        ).onSuccess {
                                            showText("Removed", it)
                                        }.onFailure {
                                            showText("Error", it.message ?: "failed")
                                        }
                                    }
                                }
                            }
                        }
                    }
                    13 -> {
                        prompt("Relay URL", "https://relay.example.com") { relayURL ->
                            prompt("Persona ID", "persona id") { personaID ->
                                prompt("Group ID", "g:...") { groupID ->
                                    prompt("Message", "text", multiline = true) { text ->
                                        Bridge.chatSendGroupText(
                                            relayURL = relayURL.trim(),
                                            context = this,
                                            personaID = personaID.trim(),
                                            groupID = groupID.trim(),
                                            text = text,
                                        ).onSuccess {
                                            showText("Sent", it)
                                        }.onFailure {
                                            showText("Error", it.message ?: "failed")
                                        }
                                    }
                                }
                            }
                        }
                    }
                    14 -> {
                        Bridge.communityList(this).onSuccess {
                            showText("Communities", it)
                        }.onFailure {
                            showText("Error", it.message ?: "failed")
                        }
                    }
                    15 -> {
                        prompt("Community ID", "leave empty to auto-generate", initial = "") { communityID ->
                            Bridge.communityCreate(this, communityID.trim()).onSuccess {
                                showText("Community created", it)
                            }.onFailure {
                                showText("Error", it.message ?: "failed")
                            }
                        }
                    }
                    16 -> {
                        prompt("Persona ID", "persona id") { personaID ->
                            prompt("Community ID", "community id") { communityID ->
                                prompt("TTL (seconds)", "e.g. 86400", initial = "86400") { ttl ->
                                    prompt("Max uses", "e.g. 1", initial = "1") { maxUses ->
                                        val ttlSec = ttl.trim().toIntOrNull() ?: 86400
                                        val maxUsesInt = maxUses.trim().toIntOrNull() ?: 1
                                        Bridge.communityCreateInvite(
                                            context = this,
                                            personaID = personaID.trim(),
                                            communityID = communityID.trim(),
                                            ttlSec = ttlSec,
                                            maxUses = maxUsesInt,
                                        ).onSuccess {
                                            showText("Invite", it)
                                        }.onFailure {
                                            showText("Error", it.message ?: "failed")
                                        }
                                    }
                                }
                            }
                        }
                    }
                    17 -> {
                        prompt("Persona ID", "persona id") { personaID ->
                            prompt("Invite", "sn4inv:...", multiline = true) { invite ->
                                Bridge.communityCreateJoinRequest(
                                    context = this,
                                    personaID = personaID.trim(),
                                    invite = invite.trim(),
                                ).onSuccess {
                                    showText("Join request", it)
                                }.onFailure {
                                    showText("Error", it.message ?: "failed")
                                }
                            }
                        }
                    }
                    18 -> {
                        prompt("Persona ID", "issuer persona id") { personaID ->
                            prompt("Invite", "sn4inv:...", multiline = true) { invite ->
                                prompt("Join request", "sn4jr:...", multiline = true) { joinRequest ->
                                    prompt("Role", "owner|moderator|member", initial = "member") { role ->
                                        Bridge.communityApproveJoin(
                                            context = this,
                                            personaID = personaID.trim(),
                                            invite = invite.trim(),
                                            joinRequest = joinRequest.trim(),
                                            role = role.trim(),
                                        ).onSuccess {
                                            showText("Approval", it)
                                        }.onFailure {
                                            showText("Error", it.message ?: "failed")
                                        }
                                    }
                                }
                            }
                        }
                    }
                    19 -> {
                        prompt("Persona ID", "applicant persona id") { personaID ->
                            prompt("Invite", "sn4inv:...", multiline = true) { invite ->
                                prompt("Join request", "sn4jr:...", multiline = true) { joinRequest ->
                                    prompt("Approval", "sn4ja:...", multiline = true) { approval ->
                                        Bridge.communityApplyJoin(
                                            context = this,
                                            personaID = personaID.trim(),
                                            invite = invite.trim(),
                                            joinRequest = joinRequest.trim(),
                                            approval = approval.trim(),
                                        ).onSuccess {
                                            showText("Joined", it)
                                        }.onFailure {
                                            showText("Error", it.message ?: "failed")
                                        }
                                    }
                                }
                            }
                        }
                    }
                    20 -> {
                        prompt("Title", "room title", initial = "general") { title ->
                            prompt("Community ID", "community id") { communityID ->
                                prompt("Room ID", "room id", initial = "general") { roomID ->
                                    prompt("Member IDs (CSV)", "contact_id1,contact_id2") { memberIDs ->
                                        Bridge.chatCreateCommunityRoom(
                                            context = this,
                                            title = title.trim(),
                                            communityID = communityID.trim(),
                                            roomID = roomID.trim(),
                                            memberIDsCSV = memberIDs.trim(),
                                        ).onSuccess {
                                            showText("Room created", it)
                                        }.onFailure {
                                            showText("Error", it.message ?: "failed")
                                        }
                                    }
                                }
                            }
                        }
                    }
                    21 -> {
                        prompt("Relay URL", "https://relay.example.com") { relayURL ->
                            prompt("Persona ID", "persona id") { personaID ->
                                prompt("Community ID", "community id") { communityID ->
                                    prompt("Room ID", "room id", initial = "general") { roomID ->
                                        prompt("Post", "text", multiline = true) { text ->
                                            Bridge.chatSendCommunityPost(
                                                relayURL = relayURL.trim(),
                                                context = this,
                                                personaID = personaID.trim(),
                                                communityID = communityID.trim(),
                                                roomID = roomID.trim(),
                                                text = text,
                                            ).onSuccess {
                                                showText("Posted", it)
                                            }.onFailure {
                                                showText("Error", it.message ?: "failed")
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                    22 -> {
                        prompt("Persona ID", "persona id") { personaID ->
                            Bridge.createDeviceJoinRequest(this, personaID.trim()).onSuccess {
                                showText("Join Request", it)
                            }.onFailure {
                                showText("Error", it.message ?: "failed")
                            }
                        }
                    }
                    23 -> {
                        prompt("Persona ID", "persona id") { personaID ->
                            prompt("Join Request", "sn4dj:...", multiline = true) { joinRequest ->
                                Bridge.approveDeviceJoinRequest(this, personaID.trim(), joinRequest.trim()).onSuccess { bundle ->
                                    val sasResult = Bridge.deviceLinkSAS(bundle)
                                    val sasMsg = if (sasResult.isSuccess) "\n\nSAS: ${sasResult.getOrNull()}" else ""
                                    showText("Link Bundle", bundle + sasMsg)
                                }.onFailure {
                                    showText("Error", it.message ?: "failed")
                                }
                            }
                        }
                    }
                    24 -> {
                        prompt("Persona ID", "persona id") { personaID ->
                            prompt("Link Bundle", "sn4dl:...", multiline = true) { joinPackage ->
                                val sasResult = Bridge.deviceLinkSAS(joinPackage.trim())
                                if (sasResult.isSuccess) {
                                    AlertDialog.Builder(this)
                                        .setTitle("Verify SAS")
                                        .setMessage("SAS: ${sasResult.getOrNull()}\n\nDo you want to apply this join package?")
                                        .setPositiveButton("Apply") { _, _ ->
                                            Bridge.applyDeviceJoinPackage(this, personaID.trim(), joinPackage.trim()).onSuccess {
                                                showText("Applied", it)
                                            }.onFailure {
                                                showText("Error", it.message ?: "failed")
                                            }
                                        }
                                        .setNegativeButton("Cancel", null)
                                        .show()
                                } else {
                                    Bridge.applyDeviceJoinPackage(this, personaID.trim(), joinPackage.trim()).onSuccess {
                                        showText("Applied", it)
                                    }.onFailure {
                                        showText("Error", it.message ?: "failed")
                                    }
                                }
                            }
                        }
                    }
                }
            }
            .setNegativeButton("Close", null)
            .show()
    }
}
