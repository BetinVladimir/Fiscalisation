package com.beeloy.fiscal.bluecash

import fi.iki.elonen.NanoHTTPD
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.security.MessageDigest
import java.time.Instant

private class LocalHttpFailure(val status:Int,message:String):RuntimeException(message)

class BlueCashLocalHttpServer(
    port: Int,
    private val adapterId: String,
    private val registerId: String,
    private val generation: Long,
    private val verifier: LocalFiscalTokenVerifier,
    private val execute: (Map<String, Any?>) -> Map<String, Any?>,
    private val lookup: (String) -> Map<String, Any?>?,
    private val fiscalReady: () -> Boolean,
    private val paymentReady: () -> Boolean,
    private val spaRoot: () -> File? = { null },
    private val deploymentState: () -> SpaDeploymentState = { SpaDeploymentState() },
) : NanoHTTPD(port) {
    override fun serve(session: IHTTPSession): Response = runCatching { route(session) }.getOrElse {
        problem(if(it is LocalHttpFailure)it.status else if (it is IllegalArgumentException) 422 else 500, it.message ?: "LOCAL_HTTP_FAILURE")
    }

    private fun route(session: IHTTPSession): Response {
        val path = session.uri.removePrefix("/beeloy/local/v1")
        if (session.method == Method.GET && path == "/healthz") {
            return json(200, mapOf("status" to "alive", "api_version" to "2026-08-14"))
        }
        if (session.method == Method.GET && path == "/deployment") {
            val value=deploymentState()
            return json(200, mapOf("application_id" to value.applicationId,"version" to value.version,"build_id" to value.buildId,"state" to value.state,"error_code" to value.errorCode))
        }

        val authority=authorize(session, if (path.contains("printer")) "fiscal.printer_test" else if (session.method == Method.GET) "fiscal.read" else "fiscal.execute")
        if (session.method == Method.GET && (path == "/readyz" || path == "/device")) {
            val fiscal = fiscalReady()
            val payment = paymentReady()
            return json(if (fiscal) 200 else 503, mapOf(
                "adapter_device_id" to adapterId,
                "register_id" to registerId,
                "binding_generation" to generation,
                "state" to if (fiscal && payment) "READY" else if (fiscal) "DEGRADED" else "OFFLINE",
                "observed_at" to Instant.now().toString(),
            ))
        }
        if (session.method == Method.POST && path == "/intents") {
            require(session.headers["x-beeloy-api-version"] == "2026-08-14") { "API_VERSION_REQUIRED" }
            val declared=session.headers["content-length"]?.toLongOrNull()?:0L
            require(declared in 1..8192) { "BODY_SIZE_INVALID" }
            val rawIntent=JSONObject(body(session))
            val canonicalPayload=rawIntent.optJSONObject("canonical_payload")?:error("CANONICAL_PAYLOAD_REQUIRED")
            val digest=MessageDigest.getInstance("SHA-256").digest(CanonicalJson.encode(canonicalPayload).toByteArray()).joinToString(""){"%02x".format(it)}
            require(digest==rawIntent.optString("payload_sha256")){"PAYLOAD_DIGEST_MISMATCH"}
            val intent = jsonObjectToMap(rawIntent)
            val id = intent["client_operation_id"] as? String
                ?: intent["intent_id"] as? String
                ?: error("OPERATION_ID_REQUIRED")
            require(session.headers["idempotency-key"] == id) { "IDEMPOTENCY_KEY_MISMATCH" }
            require(intent["tenant_id"]==authority.tenantId&&intent["location_id"]==authority.locationId&&intent["register_id"]==authority.registerId&&intent["edge_device_id"]==authority.adapterDeviceId&&(intent["binding_generation"] as? Number)?.toLong()==authority.bindingGeneration){"INTENT_BINDING_FENCE"}
            require((intent["intent_id"]?:id)==id&&(intent["action"]?:intent["command"])==intent["command"]){"INTENT_ALIAS_CONFLICT"}
            lookup(id)?.let { return json(200, it) }
            val normalized = if (intent.containsKey("action")) intent else intent + mapOf("intent_id" to id, "action" to intent["command"])
            val result = execute(normalized)
            return json(202, result)
        }
        if (session.method == Method.GET && path.startsWith("/operations/")) {
            val id = path.substringAfterLast('/')
            return lookup(id)?.let { json(200, it) } ?: problem(404, "OPERATION_NOT_FOUND")
        }
        if (session.method == Method.POST && path.startsWith("/operations/") && path.endsWith(":reconcile")) {
            val id = path.removePrefix("/operations/").removeSuffix(":reconcile")
            return lookup(id)?.let { json(200, it) } ?: problem(404, "OPERATION_NOT_FOUND")
        }
        return static(session, path)
    }

    private fun authorize(session: IHTTPSession, scope: String): LocalFiscalAuthority {
        val raw = session.headers["authorization"]?.takeIf{it.startsWith("Bearer ")}?.removePrefix("Bearer ") ?: throw LocalHttpFailure(401,"TOKEN_REQUIRED")
        return runCatching{verifier.verify(raw, scope)}.getOrElse{throw LocalHttpFailure(401,it.message?:"TOKEN_INVALID")}
    }

    private fun body(session: IHTTPSession): String {
        val files = mutableMapOf<String, String>()
        session.parseBody(files)
        return files["postData"] ?: "{}"
    }

    private fun jsonValue(value: Any?): Any? = when (value) {
        JSONObject.NULL -> null
        is JSONObject -> jsonObjectToMap(value)
        is JSONArray -> (0 until value.length()).map { jsonValue(value.get(it)) }
        else -> value
    }

    private fun jsonObjectToMap(value: JSONObject): Map<String, Any?> =
        value.keys().asSequence().associateWith { jsonValue(value.get(it)) }

    private fun static(session: IHTTPSession, path: String): Response {
        if (session.method != Method.GET) return problem(404, "NOT_FOUND")
        val relative = if (path == "/" || path.isBlank()) "index.html" else path.removePrefix("/")
        require(!relative.contains("..") && !relative.startsWith('/')) { "PATH_INVALID" }
        val root = spaRoot()?.canonicalFile ?: return problem(404,"NOT_FOUND")
        val file = File(root, relative).canonicalFile
        require(file.path.startsWith(root.path + File.separator) && file.isFile) { "NOT_FOUND" }
        return newFixedLengthResponse(Response.Status.OK, mime(relative), file.inputStream(), file.length()).apply {
            addHeader("X-Content-Type-Options", "nosniff")
            addHeader("Cache-Control", if (relative == "index.html") "no-cache" else "public,max-age=31536000,immutable")
        }
    }

    private fun json(status: Int, value: Map<String, *>): Response =
        newFixedLengthResponse(Response.Status.lookup(status), "application/json", JSONObject(value).toString()).apply {
            addHeader("Cache-Control", "no-store")
        }

    private fun problem(status: Int, code: String): Response =
        newFixedLengthResponse(
            Response.Status.lookup(status),
            "application/problem+json",
            JSONObject(mapOf("type" to "about:blank", "title" to code, "status" to status)).toString(),
        ).apply { addHeader("Cache-Control", "no-store") }

    private fun mime(path: String): String = when {
        path.endsWith(".html") -> "text/html"
        path.endsWith(".js") -> "text/javascript"
        path.endsWith(".css") -> "text/css"
        path.endsWith(".json") -> "application/json"
        else -> "application/octet-stream"
    }
}
