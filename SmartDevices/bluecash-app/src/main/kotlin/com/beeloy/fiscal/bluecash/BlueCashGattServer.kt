package com.beeloy.fiscal.bluecash

import android.annotation.SuppressLint
import android.bluetooth.BluetoothGatt
import android.bluetooth.BluetoothGattCharacteristic
import android.bluetooth.BluetoothGattServer
import android.bluetooth.BluetoothGattServerCallback
import android.bluetooth.BluetoothGattService
import android.bluetooth.BluetoothManager
import android.bluetooth.le.AdvertiseCallback
import android.bluetooth.le.AdvertiseData
import android.bluetooth.le.AdvertiseSettings
import android.content.Context
import android.os.ParcelUuid
import java.util.UUID

class BlueCashGattServer(private val context: Context, private val controller: BlueCashActivationController) {
    companion object {
        val SERVICE_UUID: UUID = UUID.fromString("7b6f1000-7c6d-4c7a-9e4f-424545464953")
        val TOKEN_UUID: UUID = UUID.fromString("7b6f1001-7c6d-4c7a-9e4f-424545464953")
        val STATUS_UUID: UUID = UUID.fromString("7b6f1002-7c6d-4c7a-9e4f-424545464953")
    }
    private val manager = context.getSystemService(BluetoothManager::class.java)
    private var server: BluetoothGattServer? = null
    private var status = "LOGIN_REQUIRED"
    private val chunks = TokenChunkAssembler()
    private val advertiseCallback = object : AdvertiseCallback() {}
    private val callback = object : BluetoothGattServerCallback() {
        @SuppressLint("MissingPermission")
        override fun onCharacteristicWriteRequest(device: android.bluetooth.BluetoothDevice, requestId: Int, characteristic: BluetoothGattCharacteristic, preparedWrite: Boolean, responseNeeded: Boolean, offset: Int, value: ByteArray) {
            val result = if (characteristic.uuid == TOKEN_UUID && offset == 0 && !preparedWrite) runCatching {
                val token = chunks.accept(device.address, value.toString(Charsets.UTF_8))
                status = if (token == null) "RECEIVING_TOKEN" else { controller.acceptToken(token); "ACTIVATED" }
            }.fold({ BluetoothGatt.GATT_SUCCESS }, { status = it.message ?: "ACTIVATION_REJECTED"; BluetoothGatt.GATT_FAILURE }) else BluetoothGatt.GATT_REQUEST_NOT_SUPPORTED
            if (responseNeeded) server?.sendResponse(device, requestId, result, 0, null)
        }
        @SuppressLint("MissingPermission")
        override fun onCharacteristicReadRequest(device: android.bluetooth.BluetoothDevice, requestId: Int, offset: Int, characteristic: BluetoothGattCharacteristic) {
            val bytes = status.toByteArray()
            val valid = characteristic.uuid == STATUS_UUID && offset <= bytes.size
            server?.sendResponse(device, requestId, if (valid) BluetoothGatt.GATT_SUCCESS else BluetoothGatt.GATT_INVALID_OFFSET, offset, if (valid) bytes.copyOfRange(offset, bytes.size) else null)
        }
    }

    @SuppressLint("MissingPermission")
    fun start() {
        require(manager.adapter?.isEnabled == true) { "BLUETOOTH_DISABLED" }
        stop()
        server = manager.openGattServer(context, callback)
        val service = BluetoothGattService(SERVICE_UUID, BluetoothGattService.SERVICE_TYPE_PRIMARY)
        service.addCharacteristic(BluetoothGattCharacteristic(TOKEN_UUID, BluetoothGattCharacteristic.PROPERTY_WRITE, BluetoothGattCharacteristic.PERMISSION_WRITE))
        service.addCharacteristic(BluetoothGattCharacteristic(STATUS_UUID, BluetoothGattCharacteristic.PROPERTY_READ, BluetoothGattCharacteristic.PERMISSION_READ))
        check(server?.addService(service) == true) { "GATT_SERVICE_FAILED" }
        val settings = AdvertiseSettings.Builder().setAdvertiseMode(AdvertiseSettings.ADVERTISE_MODE_LOW_LATENCY).setConnectable(true).setTimeout(0).build()
        val data = AdvertiseData.Builder().setIncludeDeviceName(true).addServiceUuid(ParcelUuid(SERVICE_UUID)).build()
        manager.adapter.bluetoothLeAdvertiser?.startAdvertising(settings, data, advertiseCallback) ?: error("BLE_ADVERTISING_UNAVAILABLE")
        status = "READY_FOR_TOKEN"
    }

    @SuppressLint("MissingPermission")
    fun stop() {
        manager.adapter?.bluetoothLeAdvertiser?.stopAdvertising(advertiseCallback)
        server?.close()
        server = null
        chunks.clear()
    }
}

/** BFA1|transfer-id|1-based-index|total|ASCII-JWT-fragment. */
class TokenChunkAssembler {
    private data class Transfer(val total: Int, val parts: MutableMap<Int, String> = mutableMapOf())
    private val transfers = mutableMapOf<String, Transfer>()

    @Synchronized fun accept(peer: String, frame: String): String? {
        val fields = frame.split('|', limit = 5)
        require(fields.size == 5 && fields[0] == "BFA1") { "ACTIVATION_FRAME_INVALID" }
        val transferId = fields[1]
        val index = fields[2].toIntOrNull() ?: error("ACTIVATION_FRAME_INVALID")
        val total = fields[3].toIntOrNull() ?: error("ACTIVATION_FRAME_INVALID")
        require(transferId.matches(Regex("[A-Za-z0-9-]{8,64}")) && total in 1..32 && index in 1..total && fields[4].isNotEmpty()) { "ACTIVATION_FRAME_INVALID" }
        val key = "$peer:$transferId"
        val transfer = transfers.getOrPut(key) { Transfer(total) }
        require(transfer.total == total) { "ACTIVATION_FRAME_CONFLICT" }
        val previous = transfer.parts.putIfAbsent(index, fields[4])
        require(previous == null || previous == fields[4]) { "ACTIVATION_FRAME_CONFLICT" }
        if (transfer.parts.size != total) return null
        val token = (1..total).joinToString("") { transfer.parts[it] ?: error("ACTIVATION_FRAME_INCOMPLETE") }
        transfers.remove(key)
        require(token.length <= 4096) { "ACTIVATION_TOKEN_TOO_LARGE" }
        return token
    }

    @Synchronized fun clear() = transfers.clear()
}
