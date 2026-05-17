package com.sunlionet.agent.proximity

import android.annotation.SuppressLint
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothGatt
import android.bluetooth.BluetoothGattCharacteristic
import android.bluetooth.BluetoothGattDescriptor
import android.bluetooth.BluetoothGattServer
import android.bluetooth.BluetoothGattServerCallback
import android.bluetooth.BluetoothGattService
import android.bluetooth.BluetoothManager
import android.content.Context
import com.sunlionet.agent.Logs
import java.util.UUID
import java.util.concurrent.CopyOnWriteArraySet
import java.util.concurrent.atomic.AtomicReference

class ProximityGattServer(
    context: Context,
    private val onFrame: (ByteArray) -> Unit,
) {
    private val appContext = context.applicationContext
    private val btManager = context.getSystemService(Context.BLUETOOTH_SERVICE) as BluetoothManager
    private val serverRef = AtomicReference<BluetoothGattServer?>(null)
    private val connections = CopyOnWriteArraySet<BluetoothDevice>()
    private val txCharRef = AtomicReference<BluetoothGattCharacteristic?>(null)

    private val cccdUuid: UUID = UUID.fromString("00002902-0000-1000-8000-00805f9b34fb")

    private val callback =
        object : BluetoothGattServerCallback() {
            override fun onConnectionStateChange(device: BluetoothDevice, status: Int, newState: Int) {
                if (newState == BluetoothGatt.STATE_CONNECTED) {
                    connections.add(device)
                    Logs.i("proximity", "peer connected addr=${device.address}")
                } else {
                    connections.remove(device)
                    Logs.i("proximity", "peer disconnected addr=${device.address}")
                }
            }

            override fun onCharacteristicWriteRequest(
                device: BluetoothDevice,
                requestId: Int,
                characteristic: BluetoothGattCharacteristic,
                preparedWrite: Boolean,
                responseNeeded: Boolean,
                offset: Int,
                value: ByteArray?,
            ) {
                if (characteristic.uuid == ProximityConstants.RX_UUID) {
                    if (offset == 0 && value != null) {
                        onFrame(value.copyOf())
                    }
                }
                if (responseNeeded) {
                    sendSuccess(device, requestId)
                }
            }

            override fun onDescriptorWriteRequest(
                device: BluetoothDevice,
                requestId: Int,
                descriptor: BluetoothGattDescriptor,
                preparedWrite: Boolean,
                responseNeeded: Boolean,
                offset: Int,
                value: ByteArray?,
            ) {
                if (responseNeeded) {
                    sendSuccess(device, requestId)
                }
            }
        }

    fun start(): Boolean {
        if (serverRef.get() != null) return true
        if (!ProximityBluetoothPermissions.canConnect(appContext)) {
            Logs.w("proximity", "gatt server skipped: bluetooth connect permission missing")
            return false
        }
        val server = openGattServer() ?: return false
        val svc = BluetoothGattService(ProximityConstants.SERVICE_UUID, BluetoothGattService.SERVICE_TYPE_PRIMARY)

        val rx =
            BluetoothGattCharacteristic(
                ProximityConstants.RX_UUID,
                BluetoothGattCharacteristic.PROPERTY_WRITE or BluetoothGattCharacteristic.PROPERTY_WRITE_NO_RESPONSE,
                BluetoothGattCharacteristic.PERMISSION_WRITE,
            )
        val tx =
            BluetoothGattCharacteristic(
                ProximityConstants.TX_UUID,
                BluetoothGattCharacteristic.PROPERTY_NOTIFY,
                BluetoothGattCharacteristic.PERMISSION_READ,
            )
        val cccd =
            BluetoothGattDescriptor(
                cccdUuid,
                BluetoothGattDescriptor.PERMISSION_READ or BluetoothGattDescriptor.PERMISSION_WRITE,
            )
        tx.addDescriptor(cccd)

        svc.addCharacteristic(rx)
        svc.addCharacteristic(tx)
        addService(server, svc)

        txCharRef.set(tx)
        serverRef.set(server)
        Logs.i("proximity", "gatt server started")
        return true
    }

    fun stop() {
        val server = serverRef.getAndSet(null) ?: return
        connections.clear()
        txCharRef.set(null)
        closeServer(server)
    }

    fun notifyAll(frame: ByteArray) {
        val server = serverRef.get() ?: return
        val tx = txCharRef.get() ?: return
        if (!ProximityBluetoothPermissions.canConnect(appContext)) return
        tx.value = frame
        connections.forEach { d ->
            notifyChanged(server, d, tx)
        }
    }

    @SuppressLint("MissingPermission")
    private fun openGattServer(): BluetoothGattServer? {
        return btManager.openGattServer(appContext, callback)
    }

    @SuppressLint("MissingPermission")
    private fun addService(server: BluetoothGattServer, service: BluetoothGattService) {
        server.addService(service)
    }

    @SuppressLint("MissingPermission")
    private fun sendSuccess(device: BluetoothDevice, requestId: Int) {
        runCatching { serverRef.get()?.sendResponse(device, requestId, BluetoothGatt.GATT_SUCCESS, 0, null) }
    }

    @SuppressLint("MissingPermission")
    private fun closeServer(server: BluetoothGattServer) {
        runCatching { server.close() }
    }

    @SuppressLint("MissingPermission")
    private fun notifyChanged(
        server: BluetoothGattServer,
        device: BluetoothDevice,
        tx: BluetoothGattCharacteristic,
    ) {
        runCatching { server.notifyCharacteristicChanged(device, tx, false) }
    }
}
