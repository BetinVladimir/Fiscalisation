package com.beeloy.fiscal.bluecash

import android.Manifest
import android.app.Activity
import android.os.Build
import android.os.Bundle
import android.text.InputType
import android.view.ViewGroup
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView

class MainActivity : Activity() {
    private lateinit var status: TextView
    private lateinit var organization: EditText
    private lateinit var location: EditText
    private lateinit var device: EditText
    private lateinit var pin: EditText
    private lateinit var gatt: BlueCashGattServer
    private val controller = BlueCashActivationController { state -> runOnUiThread { status.text = "Състояние: $state" } }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        gatt = BlueCashGattServer(this, controller)
        val content = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(32, 32, 32, 32) }
        content.addView(TextView(this).apply { text = "BeeFiscal BlueCash-50"; textSize = 26f })
        content.addView(TextView(this).apply { text = "Влезте локално, за да разрешите BLE activation. Токенът се издава от BeeFiscal и съдържа организация и търговска точка." })
        organization = field(content, "Organization ID")
        location = field(content, "Location ID (търговска точка)")
        device = field(content, "Fiscal device ID")
        pin = field(content, "Локален activation PIN (минимум 8 знака)").apply { inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD }
        status = TextView(this).apply { text = "Състояние: LOCKED"; textSize = 18f; setPadding(0, 24, 0, 24) }
        content.addView(status)
        content.addView(Button(this).apply { text = "Вход и включване на BLE"; setOnClickListener { loginAndAdvertise() } })
        content.addView(Button(this).apply { text = "Изход / изключване на BLE"; setOnClickListener { gatt.stop(); controller.clear() } })
        content.addView(TextView(this).apply { text = "BLE service: ${BlueCashGattServer.SERVICE_UUID}\nToken characteristic: ${BlueCashGattServer.TOKEN_UUID}\nStatus characteristic: ${BlueCashGattServer.STATUS_UUID}" })
        setContentView(ScrollView(this).apply { addView(content, ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT)) })
        requestBlePermissions()
    }

    private fun field(parent: LinearLayout, hint: String): EditText = EditText(this).also { it.hint = hint; it.minHeight = 56; parent.addView(it, ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT)) }

    private fun loginAndAdvertise() {
        runCatching {
            controller.login(LocalLogin(organization.text.toString().trim(), location.text.toString().trim(), device.text.toString().trim(), pin.text.toString()))
            gatt.start()
        }.onFailure { status.text = "Отказ: ${it.message}" }
    }

    private fun requestBlePermissions() {
        val permissions = if (Build.VERSION.SDK_INT >= 31) arrayOf(Manifest.permission.BLUETOOTH_SCAN, Manifest.permission.BLUETOOTH_CONNECT, Manifest.permission.BLUETOOTH_ADVERTISE) else arrayOf(Manifest.permission.ACCESS_FINE_LOCATION)
        requestPermissions(permissions, 1001)
    }

    override fun onDestroy() { gatt.stop(); super.onDestroy() }
}
