package com.beeloy.fiscal.bluecash

import android.annotation.SuppressLint
import android.bluetooth.*
import android.bluetooth.le.AdvertiseCallback
import android.bluetooth.le.AdvertiseData
import android.bluetooth.le.AdvertiseSettings
import android.content.Context
import android.os.ParcelUuid
import java.util.UUID

/** Android peripheral binding for the BeeFiscal BLE v1 transaction channel. */
class BlueCashTransactionGattServer(
  private val context: Context,
  private val ticketSigningKey: ByteArray,
  private val binding: BlueCashBleBinding,
  private val execute: (Map<String, Any?>) -> Map<String, Any?>
) {
  companion object {
    val SERVICE_UUID: UUID = UUID.fromString("7b6f0000-7c6d-4c7a-9e4f-424545464953")
    val CONTROL_UUID: UUID = UUID.fromString("7b6f0001-7c6d-4c7a-9e4f-424545464953")
    val COMMAND_UUID: UUID = UUID.fromString("7b6f0002-7c6d-4c7a-9e4f-424545464953")
    val EVENT_UUID: UUID = UUID.fromString("7b6f0003-7c6d-4c7a-9e4f-424545464953")
    val FLOW_UUID: UUID = UUID.fromString("7b6f0004-7c6d-4c7a-9e4f-424545464953")
    private val CCC_UUID = UUID.fromString("00002902-0000-1000-8000-00805f9b34fb")
  }
  private val manager = context.getSystemService(BluetoothManager::class.java)
  private var server: BluetoothGattServer? = null
  private lateinit var event: BluetoothGattCharacteristic
  private val channels = mutableMapOf<String, BlueCashOpenMvpChannel>()
  private val mtu = mutableMapOf<String, Int>()
  private val advertising = object : AdvertiseCallback() {}
  private val callback =
    object : BluetoothGattServerCallback() {
      override fun onConnectionStateChange(device: BluetoothDevice, status: Int, newState: Int) {
        if (newState != BluetoothProfile.STATE_CONNECTED) {
          channels.remove(device.address)?.close()
          mtu.remove(device.address)
        }
      }
      override fun onMtuChanged(device: BluetoothDevice, value: Int) {
        mtu[device.address] = value.coerceIn(64, 517)
      }
      @SuppressLint("MissingPermission")
      override fun onCharacteristicWriteRequest(
        device: BluetoothDevice,
        requestId: Int,
        characteristic: BluetoothGattCharacteristic,
        preparedWrite: Boolean,
        responseNeeded: Boolean,
        offset: Int,
        value: ByteArray
      ) {
        val result =
          runCatching {
              require(!preparedWrite && offset == 0 && characteristic.uuid == COMMAND_UUID) {
                "BLE_WRITE_MODE"
              }
              val channel =
                channels.getOrPut(device.address) {
                  BlueCashOpenMvpChannel({ intent ->
                    require(
                      intent["tenant_id"] == binding.tenantId &&
                        intent["location_id"] == binding.locationId &&
                        intent["register_id"] == binding.registerId &&
                        intent["edge_device_id"] == binding.edgeId &&
                        (intent["binding_generation"] as? Number)?.toLong() == binding.generation
                    ) {
                      "BLE_BINDING_FENCE"
                    }
                    execute(intent)
                  }) {
                    mtu[device.address] ?: 185
                  }
                }
              channel.command(value).forEach { notify(device, it) }
            }
            .fold(
              { BluetoothGatt.GATT_SUCCESS },
              {
                channels.remove(device.address)?.close()
                BluetoothGatt.GATT_FAILURE
              }
            )
        if (responseNeeded) server?.sendResponse(device, requestId, result, 0, null)
      }
      @SuppressLint("MissingPermission")
      override fun onDescriptorWriteRequest(
        device: BluetoothDevice,
        requestId: Int,
        descriptor: BluetoothGattDescriptor,
        preparedWrite: Boolean,
        responseNeeded: Boolean,
        offset: Int,
        value: ByteArray
      ) {
        val ok =
          descriptor.uuid == CCC_UUID &&
            !preparedWrite &&
            offset == 0 &&
            (value.contentEquals(BluetoothGattDescriptor.ENABLE_NOTIFICATION_VALUE) ||
              value.contentEquals(BluetoothGattDescriptor.DISABLE_NOTIFICATION_VALUE))
        if (responseNeeded)
          server?.sendResponse(
            device,
            requestId,
            if (ok) BluetoothGatt.GATT_SUCCESS else BluetoothGatt.GATT_FAILURE,
            0,
            null
          )
      }
    }
  @Suppress("DEPRECATION")
  @SuppressLint("MissingPermission")
  private fun notify(device: BluetoothDevice, value: ByteArray) {
    event.value = value
    check(server?.notifyCharacteristicChanged(device, event, false) == true) { "BLE_NOTIFY_FAILED" }
  }
  @SuppressLint("MissingPermission")
  fun start() {
    require(manager.adapter?.isEnabled == true) { "BLUETOOTH_DISABLED" }
    stop()
    server = manager.openGattServer(context, callback)
    val service = BluetoothGattService(SERVICE_UUID, BluetoothGattService.SERVICE_TYPE_PRIMARY)
    service.addCharacteristic(
      BluetoothGattCharacteristic(
        COMMAND_UUID,
        BluetoothGattCharacteristic.PROPERTY_WRITE,
        BluetoothGattCharacteristic.PERMISSION_WRITE
      )
    )
    event =
      BluetoothGattCharacteristic(EVENT_UUID, BluetoothGattCharacteristic.PROPERTY_NOTIFY, 0)
        .apply {
          addDescriptor(
            BluetoothGattDescriptor(
              CCC_UUID,
              BluetoothGattDescriptor.PERMISSION_READ or BluetoothGattDescriptor.PERMISSION_WRITE
            )
          )
        }
    service.addCharacteristic(event)
    check(server?.addService(service) == true) { "BLE_GATT_SERVICE" }
    val settings =
      AdvertiseSettings.Builder()
        .setAdvertiseMode(AdvertiseSettings.ADVERTISE_MODE_LOW_LATENCY)
        .setConnectable(true)
        .build()
    val data =
      AdvertiseData.Builder()
        .addServiceUuid(ParcelUuid(SERVICE_UUID))
        .setIncludeDeviceName(true)
        .build()
    manager.adapter.bluetoothLeAdvertiser?.startAdvertising(settings, data, advertising)
      ?: error("BLE_ADVERTISING_UNAVAILABLE")
  }
  @SuppressLint("MissingPermission")
  fun stop() {
    manager.adapter?.bluetoothLeAdvertiser?.stopAdvertising(advertising)
    channels.values.forEach { it.close() }
    channels.clear()
    mtu.clear()
    server?.close()
    server = null
  }
}
