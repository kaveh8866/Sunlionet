package com.sunlionet.agent.proximity

import android.annotation.SuppressLint
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothGatt
import android.bluetooth.BluetoothGattCallback
import android.bluetooth.BluetoothGattCharacteristic
import android.bluetooth.BluetoothGattDescriptor
import android.bluetooth.BluetoothGattService
import android.bluetooth.BluetoothProfile
import android.content.Context
import android.os.Build
import com.sunlionet.agent.Logs
import java.util.UUID
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

class ProximityGattClient(
    private val context: Context,
    private val device: BluetoothDevice,
    private val onFrame: (ByteArray) -> Unit,
    private val onReady: () -> Unit,
) {
    private val gattRef = AtomicReference<BluetoothGatt?>(null)
    private val rxRef = AtomicReference<BluetoothGattCharacteristic?>(null)
    private val txRef = AtomicReference<BluetoothGattCharacteristic?>(null)
    private val queue = ConcurrentLinkedQueue<ByteArray>()
    private val writing = AtomicBoolean(false)
    private val ready = AtomicBoolean(false)

    private val cccdUuid: UUID = UUID.fromString("00002902-0000-1000-8000-00805f9b34fb")
    private val enableNotify = byteArrayOf(0x01, 0x00)
    private val appContext = context.applicationContext

    private val cb =
        object : BluetoothGattCallback() {
            override fun onConnectionStateChange(gatt: BluetoothGatt, status: Int, newState: Int) {
                if (newState == BluetoothProfile.STATE_CONNECTED) {
                    discoverServices(gatt)
                } else {
                    ready.set(false)
                    rxRef.set(null)
                    txRef.set(null)
                    Logs.i("proximity", "disconnected addr=${device.address}")
                    closeGatt(gatt)
                    gattRef.compareAndSet(gatt, null)
                }
            }

            override fun onServicesDiscovered(gatt: BluetoothGatt, status: Int) {
                if (status != BluetoothGatt.GATT_SUCCESS) return
                val svc: BluetoothGattService = gatt.getService(ProximityConstants.SERVICE_UUID) ?: return
                val rx = svc.getCharacteristic(ProximityConstants.RX_UUID) ?: return
                val tx = svc.getCharacteristic(ProximityConstants.TX_UUID)
                rxRef.set(rx)
                txRef.set(tx)
                ready.set(true)
                if (tx != null) {
                    subscribeTx(gatt, tx)
                }
                runCatching { onReady() }
                flush()
            }

            override fun onCharacteristicChanged(gatt: BluetoothGatt, characteristic: BluetoothGattCharacteristic) {
                if (characteristic.uuid == ProximityConstants.TX_UUID) {
                    val v = characteristic.value ?: return
                    onFrame(v.copyOf())
                }
            }

            override fun onCharacteristicWrite(gatt: BluetoothGatt, characteristic: BluetoothGattCharacteristic, status: Int) {
                writing.set(false)
                flush()
            }
        }

    fun connect() {
        if (gattRef.get() != null) return
        if (!ProximityBluetoothPermissions.canConnect(appContext)) {
            Logs.w("proximity", "connect skipped: bluetooth connect permission missing")
            return
        }
        val gatt = connectGatt() ?: return
        gattRef.set(gatt)
        Logs.i("proximity", "connect addr=${device.address}")
    }

    fun disconnect() {
        val gatt = gattRef.getAndSet(null) ?: return
        ready.set(false)
        rxRef.set(null)
        txRef.set(null)
        disconnectGatt(gatt)
        closeGatt(gatt)
    }

    fun enqueueWrite(frame: ByteArray) {
        queue.add(frame.copyOf())
        flush()
    }

    private fun flush() {
        if (!ready.get()) return
        if (!writing.compareAndSet(false, true)) return
        val gatt = gattRef.get()
        val rx = rxRef.get()
        val next = queue.poll()
        if (gatt == null || rx == null || next == null) {
            writing.set(false)
            return
        }
        rx.value = next
        if (!ProximityBluetoothPermissions.canConnect(appContext)) {
            writing.set(false)
            Logs.w("proximity", "write skipped: bluetooth connect permission missing")
            return
        }
        val ok = writeCharacteristic(gatt, rx)
        if (!ok) {
            writing.set(false)
            Logs.w("proximity", "write failed addr=${device.address}")
        }
    }

    private fun subscribeTx(gatt: BluetoothGatt, tx: BluetoothGattCharacteristic) {
        if (!ProximityBluetoothPermissions.canConnect(appContext)) return
        setCharacteristicNotification(gatt, tx)
        val d: BluetoothGattDescriptor = tx.getDescriptor(cccdUuid) ?: return
        d.value = enableNotify
        writeDescriptor(gatt, d)
    }

    @SuppressLint("MissingPermission")
    private fun connectGatt(): BluetoothGatt? {
        return if (Build.VERSION.SDK_INT >= 23) {
            device.connectGatt(context, false, cb, BluetoothDevice.TRANSPORT_LE)
        } else {
            device.connectGatt(context, false, cb)
        }
    }

    @SuppressLint("MissingPermission")
    private fun discoverServices(gatt: BluetoothGatt) {
        runCatching { gatt.discoverServices() }
            .onFailure { Logs.w("proximity", "discover failed ${it.message.orEmpty()}") }
    }

    @SuppressLint("MissingPermission")
    private fun disconnectGatt(gatt: BluetoothGatt) {
        runCatching { gatt.disconnect() }
    }

    @SuppressLint("MissingPermission")
    private fun closeGatt(gatt: BluetoothGatt) {
        runCatching { gatt.close() }
    }

    @SuppressLint("MissingPermission")
    private fun writeCharacteristic(gatt: BluetoothGatt, rx: BluetoothGattCharacteristic): Boolean {
        return runCatching { gatt.writeCharacteristic(rx) }.getOrDefault(false)
    }

    @SuppressLint("MissingPermission")
    private fun setCharacteristicNotification(gatt: BluetoothGatt, tx: BluetoothGattCharacteristic) {
        gatt.setCharacteristicNotification(tx, true)
    }

    @SuppressLint("MissingPermission")
    private fun writeDescriptor(gatt: BluetoothGatt, d: BluetoothGattDescriptor) {
        gatt.writeDescriptor(d)
    }
}
