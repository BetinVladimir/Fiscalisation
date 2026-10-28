package com.beeloy.fiscal.bluecash

import android.content.Context
import fi.iki.elonen.NanoHTTPD
import java.util.Base64
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

/**
 * Owns the operational runtime (MQTT, BLE GATT, local HTTP).
 * Extracted from MainActivity so that it survives Activity recreation inside a foreground Service.
 * No Activity context; accepts application/service Context only.
 */
class RuntimeHost(
  private val context: Context,
  private val journal: AndroidTransactionJournal,
  private val processor: BlueCashCommandProcessor,
  private val identity: AndroidDeviceIdentity
) {
  private var operationalMqtt: BlueCashMqttRuntime? = null
  private var transactionGatt: BlueCashTransactionGattServer? = null
  private var localHttp: BlueCashLocalHttpServer? = null
  private var deploymentManager: SpaDeploymentManager? = null
  private val deploymentScheduler = Executors.newSingleThreadScheduledExecutor()

  @Volatile private var running = false

  fun isActive(): Boolean = running

  fun start(binding: ActivatedBinding, op: Int, password: String, till: Int) {
    check(!running) { "RUNTIME_ALREADY_RUNNING" }
    running = true
    journal.provisionUnpRange(binding.unpPrefix, binding.unpRangeStart, binding.unpRangeEnd)
    val decoder = Base64.getUrlDecoder()
    val executor = BlueCashComplianceIntentExecutor(journal, journal, processor, op, password, till)
    operationalMqtt =
      BlueCashMqttRuntime(
          BlueCashMqttConfig(
            brokerUri = binding.mqttTlsUri,
            clientId = "beefiscal-${binding.deviceId}",
            tenantId = binding.organizationId,
            deviceId = binding.deviceId,
            registerId = binding.registerId,
            bindingGeneration = binding.bindingVersion,
            commandHmacKey = decoder.decode(binding.commandHmacKey),
            syncAckHmacKey = decoder.decode(binding.syncAckHmacKey),
            socketFactory = DeviceTLS.socketFactory(binding),
            fiscalOperatorNumber = op,
            fiscalOperatorPassword = password,
            tillNumber = till
          ),
          processor,
          journal,
          identity,
          executor::execute
        )
        .also { it.start() }
    transactionGatt =
      BlueCashTransactionGattServer(
          context,
          decoder.decode(binding.bleTicketHmacKey),
          BlueCashBleBinding(
            binding.organizationId,
            binding.locationId,
            binding.registerId,
            binding.deviceId,
            binding.deviceId,
            binding.bindingVersion
          ),
          executor::execute
        )
        .also { it.start() }
    val issuer = binding.localTokenIssuer
    val kid = binding.localTokenSigningKID
    val publicKey = binding.localTokenPublicKeyDERBase64
    if (!issuer.isNullOrBlank() && !kid.isNullOrBlank() && !publicKey.isNullOrBlank()) {
      val verifier =
        LocalFiscalTokenVerifier(
          issuer, publicKey, kid,
          binding.organizationId, binding.locationId,
          binding.registerId, binding.deviceId, binding.bindingVersion
        )
      deploymentManager =
        SpaDeploymentManager(
            context,
            binding.spaDeploymentDescriptorURL,
            binding.spaDeploymentSigningKID,
            binding.spaDeploymentPublicKeyDERBase64
          )
          .also { manager ->
            deploymentScheduler.scheduleWithFixedDelay(
              { manager.checkAndActivate() }, 0, 6, TimeUnit.HOURS
            )
          }
      val manager = deploymentManager!!
      localHttp =
        BlueCashLocalHttpServer(
            8088,
            binding.deviceId,
            binding.registerId,
            binding.bindingVersion,
            verifier,
            executor::execute,
            { operationResult(it) },
            processor::fiscalReachable,
            processor::paymentReachable,
            manager::activeRoot,
            manager::state
          )
          .also { it.start(NanoHTTPD.SOCKET_READ_TIMEOUT, false) }
    }
  }

  private fun operationResult(id: String): Map<String, String> =
    journal.find(id).lastOrNull()?.let {
      mapOf("state" to it.type, "payload" to it.payload)
    } ?: mapOf("state" to "NOT_FOUND")

  fun stop() {
    running = false
    deploymentScheduler.shutdownNow()
    runCatching { operationalMqtt?.stop() }
    runCatching { transactionGatt?.stop() }
    runCatching { localHttp?.stop() }
    operationalMqtt = null
    transactionGatt = null
    localHttp = null
  }
}
