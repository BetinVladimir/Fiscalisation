package com.beeloy.fiscal.bluecash

data class LocalLogin(val organizationId: String, val locationId: String, val deviceId: String, val activationPin: String)

class BlueCashActivationController(private val onState: (String) -> Unit) {
    private var login: LocalLogin? = null
    var activationToken: String? = null
        private set

    fun login(value: LocalLogin) {
        require(value.organizationId.isNotBlank() && value.locationId.isNotBlank() && value.deviceId.isNotBlank()) { "IDENTITY_REQUIRED" }
        require(value.activationPin.length >= 8) { "ACTIVATION_PIN_TOO_SHORT" }
        login = value
        onState("BLE_READY")
    }

    fun acceptToken(jwt: String): ActivationClaims {
        val local = login ?: error("LOGIN_REQUIRED")
        val claims = ActivationToken.parseAndBind(jwt, local.organizationId, local.locationId, local.deviceId)
        activationToken = jwt
        onState("ACTIVATED:${claims.organizationId}:${claims.locationId}")
        return claims
    }

    fun clear() {
        login = null
        activationToken = null
        onState("LOCKED")
    }
}
