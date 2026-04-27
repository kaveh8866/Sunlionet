package com.sunlionet.agent.proximity

import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothManager
import android.bluetooth.le.AdvertiseCallback
import android.bluetooth.le.AdvertiseData
import android.bluetooth.le.AdvertiseSettings
import android.bluetooth.le.BluetoothLeAdvertiser
import android.content.Context
import android.os.Build
import android.os.ParcelUuid
import com.sunlionet.agent.Logs

class ProximityBleAdvertiser(
    context: Context,
) {
    private val adapter: BluetoothAdapter? =
        (context.getSystemService(Context.BLUETOOTH_SERVICE) as BluetoothManager).adapter
    private val advertiser: BluetoothLeAdvertiser? = adapter?.bluetoothLeAdvertiser

    private var callback: AdvertiseCallback? = null

    fun start(identity: ProximityIdentityManager.Identity) {
        val adv = advertiser ?: return
        stop()
        val settings = AdvertiseSettings.Builder()
            .setAdvertiseMode(AdvertiseSettings.ADVERTISE_MODE_LOW_POWER)
            .setTxPowerLevel(AdvertiseSettings.ADVERTISE_TX_POWER_LOW)
            .setConnectable(true)
            .build()

        val data = AdvertiseData.Builder()
            .setIncludeDeviceName(false)
            .addServiceUuid(ParcelUuid(ProximityConstants.SERVICE_UUID))
            .addServiceData(ParcelUuid(ProximityConstants.SERVICE_UUID), identity.nodeId.copyOf())
            .build()

        val cb = object : AdvertiseCallback() {}
        callback = cb
        if (Build.VERSION.SDK_INT >= 31) {
            runCatching { adv.startAdvertising(settings, data, cb) }
                .onFailure { Logs.w("proximity", "adv start failed ${it.message.orEmpty()}") }
            return
        }
        runCatching { adv.startAdvertising(settings, data, cb) }
            .onFailure { Logs.w("proximity", "adv start failed ${it.message.orEmpty()}") }
    }

    fun stop() {
        val adv = advertiser ?: return
        val cb = callback ?: return
        callback = null
        runCatching { adv.stopAdvertising(cb) }
    }
}
