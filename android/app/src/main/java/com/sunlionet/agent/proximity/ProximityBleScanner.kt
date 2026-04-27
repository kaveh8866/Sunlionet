package com.sunlionet.agent.proximity

import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothManager
import android.bluetooth.le.BluetoothLeScanner
import android.bluetooth.le.ScanCallback
import android.bluetooth.le.ScanFilter
import android.bluetooth.le.ScanResult
import android.bluetooth.le.ScanSettings
import android.content.Context
import android.os.Build
import android.os.ParcelUuid
import com.sunlionet.agent.Logs
import java.util.concurrent.atomic.AtomicBoolean

class ProximityBleScanner(
    context: Context,
    private val onSeen: (deviceAddress: String, nodeId: ByteArray, rssi: Int) -> Unit,
) {
    private val adapter: BluetoothAdapter? =
        (context.getSystemService(Context.BLUETOOTH_SERVICE) as BluetoothManager).adapter
    private val scanner: BluetoothLeScanner? = adapter?.bluetoothLeScanner
    private val running = AtomicBoolean(false)

    private val cb =
        object : ScanCallback() {
            override fun onScanResult(callbackType: Int, result: ScanResult?) {
                if (result == null) return
                val record = result.scanRecord ?: return
                val svc = record.getServiceData(ParcelUuid(ProximityConstants.SERVICE_UUID)) ?: return
                if (svc.size < ProximityConstants.ADV_NODE_ID_LEN) return
                val nodeId = svc.copyOfRange(0, ProximityConstants.ADV_NODE_ID_LEN)
                val addr = result.device?.address ?: return
                onSeen(addr, nodeId, result.rssi)
            }

            override fun onBatchScanResults(results: MutableList<ScanResult>?) {
                if (results == null) return
                results.forEach { onScanResult(0, it) }
            }
        }

    fun startLowPower() {
        val s = scanner ?: return
        if (!running.compareAndSet(false, true)) return
        val filter =
            ScanFilter.Builder()
                .setServiceUuid(ParcelUuid(ProximityConstants.SERVICE_UUID))
                .build()
        val settings =
            ScanSettings.Builder()
                .setScanMode(ScanSettings.SCAN_MODE_LOW_POWER)
                .apply {
                    if (Build.VERSION.SDK_INT >= 23) {
                        setCallbackType(ScanSettings.CALLBACK_TYPE_ALL_MATCHES)
                    }
                }
                .build()
        runCatching { s.startScan(listOf(filter), settings, cb) }
            .onFailure { Logs.w("proximity", "scan start failed ${it.message.orEmpty()}") }
    }

    fun stop() {
        val s = scanner ?: return
        if (!running.compareAndSet(true, false)) return
        runCatching { s.stopScan(cb) }
            .onFailure { Logs.w("proximity", "scan stop failed ${it.message.orEmpty()}") }
    }
}
