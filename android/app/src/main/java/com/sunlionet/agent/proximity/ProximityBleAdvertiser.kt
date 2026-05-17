package com.sunlionet.agent.proximity

import android.annotation.SuppressLint
import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothManager
import android.bluetooth.le.AdvertiseCallback
import android.bluetooth.le.AdvertiseData
import android.bluetooth.le.AdvertiseSettings
import android.bluetooth.le.BluetoothLeAdvertiser
import android.content.Context
import android.os.ParcelUuid
import com.sunlionet.agent.Logs

class ProximityBleAdvertiser(
    context: Context,
) {
    private val appContext = context.applicationContext
    private val adapter: BluetoothAdapter? =
        (context.getSystemService(Context.BLUETOOTH_SERVICE) as BluetoothManager).adapter
    private val advertiser: BluetoothLeAdvertiser? = adapter?.bluetoothLeAdvertiser

    private var callback: AdvertiseCallback? = null

    fun start(serviceData: ByteArray) {
        if (!ProximityBluetoothPermissions.canAdvertise(appContext)) {
            Logs.w("proximity", "advertising skipped: bluetooth advertise permission missing")
            return
        }
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
            .addServiceData(ParcelUuid(ProximityConstants.SERVICE_UUID), serviceData.copyOf())
            .build()

        val cb = object : AdvertiseCallback() {}
        callback = cb
        startAdvertising(adv, settings, data, cb)
    }

    fun stop() {
        if (!ProximityBluetoothPermissions.canAdvertise(appContext)) return
        val adv = advertiser ?: return
        val cb = callback ?: return
        callback = null
        stopAdvertising(adv, cb)
    }

    @SuppressLint("MissingPermission")
    private fun startAdvertising(
        adv: BluetoothLeAdvertiser,
        settings: AdvertiseSettings,
        data: AdvertiseData,
        cb: AdvertiseCallback,
    ) {
        runCatching { adv.startAdvertising(settings, data, cb) }
            .onFailure { Logs.w("proximity", "adv start failed ${it.message.orEmpty()}") }
    }

    @SuppressLint("MissingPermission")
    private fun stopAdvertising(adv: BluetoothLeAdvertiser, cb: AdvertiseCallback) {
        runCatching { adv.stopAdvertising(cb) }
    }
}
