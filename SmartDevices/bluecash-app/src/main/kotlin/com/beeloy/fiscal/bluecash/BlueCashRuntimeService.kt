package com.beeloy.fiscal.bluecash

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.IBinder
import org.json.JSONObject

/**
 * Foreground Service that keeps the operational runtime alive across Activity recreation.
 * onDestroy is only called when the Service itself is destroyed (not on configuration changes).
 * BUG-032: runtimes must not stop on Activity orientation change or recreation.
 */
class BlueCashRuntimeService : Service() {
  companion object {
    private const val NOTIFICATION_ID = 1
    private const val CHANNEL_ID = "beefiscal_runtime"

    fun bindingFromJson(result: JSONObject): ActivatedBinding {
      val roles = result.getJSONArray("roles").let { arr ->
        (0 until arr.length()).map { arr.getString(it) }
      }
      return ActivatedBinding(
        result.getString("credential_id"),
        result.optString("client_certificate_pem").ifBlank { "" },
        result.optString("ca_chain_pem").ifBlank { "" },
        result.getString("mqtt_tls_uri"),
        result.optString("mqtt_wss_uri").ifBlank { null },
        result.getString("device_instance_id"),
        result.getString("organization_id"),
        result.getString("location_id"),
        result.getString("register_id"),
        roles,
        result.getLong("binding_version"),
        result.getString("command_hmac_key"),
        result.getString("sync_ack_hmac_key"),
        result.getString("ble_ticket_hmac_key"),
        result.getString("unp_prefix"),
        result.getLong("unp_range_start"),
        result.getLong("unp_range_end"),
        result.optString("local_token_issuer").ifBlank { null },
        result.optString("local_token_signing_kid").ifBlank { null },
        result.optString("local_token_public_key_der_base64").ifBlank { null },
        result.optString("spa_deployment_descriptor_url").ifBlank { null },
        result.optString("spa_deployment_signing_kid").ifBlank { null },
        result.optString("spa_deployment_public_key_der_base64").ifBlank { null }
      )
    }
  }

  private var runtimeHost: RuntimeHost? = null

  override fun onBind(intent: Intent?): IBinder? = null

  override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
    ensureNotificationChannel()
    startForeground(NOTIFICATION_ID, buildNotification())
    if (runtimeHost?.isActive() != true) {
      val identity = AndroidDeviceIdentity(this)
      val secrets = DeviceCredentialStore(this)
      val secretStore = SecretStore(this)
      val op = secretStore.getOperatorNumber() ?: return START_STICKY
      val password = secretStore.getOperatorPassword() ?: return START_STICKY
      val till = secretStore.getTillNumber() ?: return START_STICKY
      val binding = readBinding(secrets) ?: return START_STICKY
      val journal = AndroidTransactionJournal(this, identity)
      val fiscalPort = DatecsAndroidFiscalPort(this)
      val processor = BlueCashCommandProcessor(
        fiscalPort,
        DatecsAndroidPaymentPort(this, BoricaPinpadCodec(store = journal)),
        journal
      )
      runtimeHost = RuntimeHost(this, journal, processor, identity)
        .also { it.start(binding, op, password, till) }
    }
    return START_STICKY
  }

  override fun onDestroy() {
    runtimeHost?.stop()
    runtimeHost = null
    super.onDestroy()
  }

  private fun ensureNotificationChannel() {
    val nm = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
    if (nm.getNotificationChannel(CHANNEL_ID) == null) {
      nm.createNotificationChannel(
        NotificationChannel(CHANNEL_ID, "BeeFiscal Runtime", NotificationManager.IMPORTANCE_LOW)
      )
    }
  }

  private fun buildNotification(): Notification =
    Notification.Builder(this, CHANNEL_ID)
      .setContentTitle("BeeFiscal BlueCash")
      .setContentText("Fiscal runtime active")
      .setSmallIcon(android.R.drawable.ic_dialog_info)
      .setOngoing(true)
      .build()

  private fun readBinding(secrets: DeviceCredentialStore): ActivatedBinding? =
    runCatching {
      val json = secrets.get("binding") ?: return null
      bindingFromJson(JSONObject(json))
    }.getOrNull()
}
