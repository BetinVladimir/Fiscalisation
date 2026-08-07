package com.beeloy.fiscal

data class Command(
    val operationId: String,
    val type: String,
    val payload: Map<String, Any?>,
)

data class Result(
    val operationId: String,
    val state: String,
    val fiscalReference: String? = null,
    val errorCode: String? = null,
    val simulated: Boolean = false,
    val data: Map<String, String> = emptyMap(),
)

interface FiscalDriver {
    fun probe(): Map<String, String>
    fun execute(command: Command): Result
    fun capabilities(): Map<String, String>
}
