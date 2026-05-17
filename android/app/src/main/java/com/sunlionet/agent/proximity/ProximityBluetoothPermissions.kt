package com.sunlionet.agent.proximity

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.content.ContextCompat

internal object ProximityBluetoothPermissions {
    fun canScan(context: Context): Boolean {
        return if (Build.VERSION.SDK_INT >= 31) {
            granted(context, Manifest.permission.BLUETOOTH_SCAN)
        } else {
            granted(context, Manifest.permission.ACCESS_FINE_LOCATION)
        }
    }

    fun canAdvertise(context: Context): Boolean {
        return Build.VERSION.SDK_INT < 31 || granted(context, Manifest.permission.BLUETOOTH_ADVERTISE)
    }

    fun canConnect(context: Context): Boolean {
        return Build.VERSION.SDK_INT < 31 || granted(context, Manifest.permission.BLUETOOTH_CONNECT)
    }

    private fun granted(context: Context, permission: String): Boolean {
        return ContextCompat.checkSelfPermission(context, permission) == PackageManager.PERMISSION_GRANTED
    }
}
