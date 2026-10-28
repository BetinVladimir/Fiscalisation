package com.beeloy.fiscal.bluecash

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

/**
 * Starts BlueCashRuntimeService on BOOT_COMPLETED if the device has been activated.
 * BUG-032: after reboot the runtime must resume without manual interaction.
 */
class BootReceiver : BroadcastReceiver() {
  override fun onReceive(context: Context, intent: Intent) {
    if (intent.action != Intent.ACTION_BOOT_COMPLETED) return
    val secretStore = SecretStore(context)
    if (!secretStore.isConfigured()) return
    val credentials = DeviceCredentialStore(context)
    if (credentials.get("binding") == null) return
    context.startForegroundService(Intent(context, BlueCashRuntimeService::class.java))
  }
}
