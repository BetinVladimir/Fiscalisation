package com.beeloy.fiscal.bluecash

import android.Manifest
import android.app.Activity
import android.graphics.Bitmap
import android.os.Build
import android.os.Bundle
import android.view.ViewGroup
import android.widget.*
import com.google.zxing.BarcodeFormat
import com.google.zxing.MultiFormatWriter
import fi.iki.elonen.NanoHTTPD
import java.util.Base64
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

class MainActivity : Activity() {
    private lateinit var status: TextView
    private lateinit var qr: ImageView
    private lateinit var code: TextView
    private lateinit var serial: EditText
    private lateinit var fmin: EditText
    private lateinit var firmware: EditText
    private lateinit var operatorNumber: EditText
    private lateinit var operatorPassword: EditText
    private lateinit var tillNumber: EditText
    private lateinit var journal: AndroidTransactionJournal
    private lateinit var fiscalPort: DatecsAndroidFiscalPort
    private lateinit var identity: AndroidDeviceIdentity
    private lateinit var secrets: DeviceCredentialStore
    private lateinit var processor: BlueCashCommandProcessor
    private var activationMqtt: DeviceActivationMqtt? = null
    private var operationalMqtt: BlueCashMqttRuntime? = null
    private var transactionGatt: BlueCashTransactionGattServer? = null
    private var localHttp: BlueCashLocalHttpServer? = null
    private var deploymentManager: SpaDeploymentManager? = null
    private val deploymentScheduler=Executors.newSingleThreadScheduledExecutor()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        identity = AndroidDeviceIdentity(this)
        journal = AndroidTransactionJournal(this, identity)
        fiscalPort = DatecsAndroidFiscalPort(this)
        processor = BlueCashCommandProcessor(fiscalPort, DatecsAndroidPaymentPort(this, BoricaPinpadCodec(store=journal)), journal)
        secrets = DeviceCredentialStore(this)
        val content = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(32, 32, 32, 32) }
        content.addView(TextView(this).apply { text = "BeeFiscal BlueCash-50"; textSize = 26f })
        content.addView(TextView(this).apply { text = "Устройството създава собствен неекспортируем ключ. Организацията не се въвежда тук — тя се определя от профила на администратора в BeeFiscalApp." })
        serial = field(content, "BlueCash serial number")
        fmin = field(content, "Fiscal memory number")
        firmware = field(content, "Firmware version", Build.DISPLAY)
        operatorNumber = field(content, "Fiscal operator number", "1")
        operatorPassword = field(content, "Fiscal operator password")
        tillNumber = field(content, "Till number", "1")
        status = TextView(this).apply { text = "Състояние: NOT_ACTIVATED"; textSize = 18f; setPadding(0, 24, 0, 16) }
        code = TextView(this).apply { textSize = 24f; setPadding(0, 8, 0, 8) }
        qr = ImageView(this).apply { adjustViewBounds = true }
        content.addView(status); content.addView(code); content.addView(qr, ViewGroup.LayoutParams(640, 640))
        content.addView(Button(this).apply { text = "Създай заявка за активация"; setOnClickListener { beginActivation() } })
        content.addView(Button(this).apply { text = "Провери потвърждението"; setOnClickListener { pollCredential() } })
        content.addView(TextView(this).apply { text = "Device instance: ${identity.deviceInstanceId}\nBackend URL се инжектира от подписаната DEV/PROD build pipeline. MQTT/BLE операции не стартират преди ACTIVE acknowledgement." })
        setContentView(ScrollView(this).apply { addView(content, ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT)) })
        requestBlePermissions()
    }

    private fun field(parent: LinearLayout, hint: String, initial: String = ""): EditText = EditText(this).also { it.hint = hint; it.setText(initial); it.minHeight = 56; parent.addView(it, ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT)) }
    private fun client(): DeviceBootstrapClient { check(BuildConfig.FISCAL_BACKEND_URL.startsWith("https://")) { "SIGNED_BUILD_BACKEND_NOT_CONFIGURED" }; return DeviceBootstrapClient(BuildConfig.FISCAL_BACKEND_URL, identity, secrets) }
    private fun beginActivation() = background("CREATING_REQUEST") {
        require(serial.text.toString().isNotBlank() && fmin.text.toString().isNotBlank()) { "SERIAL_AND_FMIN_REQUIRED" }
        val pending = client().begin("DATECS", "BLUECASH_50", serial.text.toString().trim(), fmin.text.toString().trim(), firmware.text.toString().trim(), listOf("FISCAL_DEVICE", "PAYMENT_TERMINAL"))
        val payload = "${pending.verificationUri}?user_code=${pending.userCode}&activation_request_id=${pending.requestId}"
        runOnUiThread { code.text = pending.userCode; qr.setImageBitmap(qrBitmap(payload, 640)); status.text = "Състояние: WAITING_FOR_ADMIN (${pending.expiresAt})" }
    }
    private fun pollCredential() = background("CHECKING_CONFIRMATION") {
        val binding = client().pollCredential()
        activationMqtt = DeviceActivationMqtt(identity, secrets, binding) { active -> startManagedRuntime(active) }.also { it.start() }
        runOnUiThread { status.text = "Състояние: CREDENTIAL_ISSUED; WAITING_FOR_SIGNED_ACTIVE_ACK\nOrganization: ${binding.organizationId}\nLocation: ${binding.locationId}\nRegister: ${binding.registerId}" }
    }
    private fun startManagedRuntime(binding: ActivatedBinding) {
        val op = operatorNumber.text.toString().toIntOrNull() ?: error("OPERATOR_NUMBER_REQUIRED")
        val till = tillNumber.text.toString().toIntOrNull() ?: error("TILL_NUMBER_REQUIRED")
        val password = operatorPassword.text.toString(); require(password.isNotBlank()) { "OPERATOR_PASSWORD_REQUIRED" }
        journal.provisionUnpRange(binding.unpPrefix, binding.unpRangeStart, binding.unpRangeEnd)
        val decoder = Base64.getUrlDecoder()
        val executor = BlueCashComplianceIntentExecutor(journal,journal,processor,op,password,till)
        operationalMqtt = BlueCashMqttRuntime(BlueCashMqttConfig(brokerUri=binding.mqttTlsUri,clientId="beefiscal-${binding.deviceId}",tenantId=binding.organizationId,deviceId=binding.deviceId,registerId=binding.registerId,bindingGeneration=binding.bindingVersion,commandHmacKey=decoder.decode(binding.commandHmacKey),syncAckHmacKey=decoder.decode(binding.syncAckHmacKey),socketFactory=DeviceTLS.socketFactory(binding),fiscalOperatorNumber=op,fiscalOperatorPassword=password,tillNumber=till),processor,journal,identity,executor::execute).also { it.start() }
        transactionGatt = BlueCashTransactionGattServer(this,decoder.decode(binding.bleTicketHmacKey),BlueCashBleBinding(binding.organizationId,binding.locationId,binding.registerId,binding.deviceId,binding.deviceId,binding.bindingVersion),executor::execute).also { it.start() }
        val issuer=binding.localTokenIssuer
        val kid=binding.localTokenSigningKID
        val publicKey=binding.localTokenPublicKeyDERBase64
        if(!issuer.isNullOrBlank()&&!kid.isNullOrBlank()&&!publicKey.isNullOrBlank()){
            val verifier=LocalFiscalTokenVerifier(issuer,publicKey,kid,binding.organizationId,binding.locationId,binding.registerId,binding.deviceId,binding.bindingVersion)
            deploymentManager=SpaDeploymentManager(this,binding.spaDeploymentDescriptorURL,binding.spaDeploymentSigningKID,binding.spaDeploymentPublicKeyDERBase64).also{manager->deploymentScheduler.scheduleWithFixedDelay({manager.checkAndActivate()},0,6,TimeUnit.HOURS)}
            val manager=deploymentManager!!
            localHttp=BlueCashLocalHttpServer(8088,binding.deviceId,binding.registerId,binding.bindingVersion,verifier,executor::execute,{operationResult(it)},processor::fiscalReachable,processor::paymentReachable,manager::activeRoot,manager::state).also{it.start(NanoHTTPD.SOCKET_READ_TIMEOUT,false)}
        }
        runOnUiThread { status.text = "Състояние: ACTIVE\nMQTT mTLS + direct BLE enabled" }
    }
    private fun background(state: String, work: () -> Unit) { status.text = "Състояние: $state"; Thread { runCatching(work).onFailure { error -> runOnUiThread { status.text = "Отказ: ${error.message}" } } }.start() }
    private fun qrBitmap(value: String, size: Int): Bitmap { val matrix = MultiFormatWriter().encode(value, BarcodeFormat.QR_CODE, size, size); return Bitmap.createBitmap(size, size, Bitmap.Config.RGB_565).also { image -> for (x in 0 until size) for (y in 0 until size) image.setPixel(x, y, if (matrix[x, y]) android.graphics.Color.BLACK else android.graphics.Color.WHITE) } }
    private fun requestBlePermissions() { val permissions = if (Build.VERSION.SDK_INT >= 31) arrayOf(Manifest.permission.BLUETOOTH_SCAN, Manifest.permission.BLUETOOTH_CONNECT, Manifest.permission.BLUETOOTH_ADVERTISE) else arrayOf(Manifest.permission.ACCESS_FINE_LOCATION); requestPermissions(permissions, 1001) }
    private fun operationResult(id:String):Map<String,Any?>?{val row=journal.find(id).lastOrNull{it.type in setOf("FISCALIZED","REVERSED","PRINTER_TESTED","FAILED","UNKNOWN","COMPENSATED","RECOVERY_REQUIRED")}?:return null;val values=row.payload.split('&').mapNotNull{part->part.split('=',limit=2).takeIf{it.size==2}?.let{it[0] to it[1]}}.toMap().toMutableMap<String,Any?>();values.putIfAbsent("state",row.type);values["operation_id"]=id;return values}
    override fun onDestroy() { deploymentScheduler.shutdownNow();localHttp?.stop(); transactionGatt?.stop(); operationalMqtt?.stop(); activationMqtt?.stop(); fiscalPort.close(); journal.close(); super.onDestroy() }
}
