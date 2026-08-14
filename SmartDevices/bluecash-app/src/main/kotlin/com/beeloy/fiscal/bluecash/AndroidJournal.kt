package com.beeloy.fiscal.bluecash

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.Signature
import java.time.Instant
import java.util.Base64
import org.json.JSONArray
import org.json.JSONObject

class AndroidKeystoreSigner(private val alias: String = "beefiscal-bluecash-transaction-v1") :
  TransactionSigner {
  private val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
  override val keyId: String
    get() = alias
  private fun key() =
    store.getKey(alias, null)
      ?: run {
        val generator =
          KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, "AndroidKeyStore")
        val builder =
          KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_SIGN)
            .setDigests(KeyProperties.DIGEST_SHA256)
            .setAlgorithmParameterSpec(java.security.spec.ECGenParameterSpec("secp256r1"))
        runCatching { builder.setIsStrongBoxBacked(true) }
        generator.initialize(builder.build())
        generator.generateKeyPair().private
      }
  override fun sign(hash: ByteArray): ByteArray =
    Signature.getInstance("SHA256withECDSA").run {
      initSign(key() as java.security.PrivateKey)
      update(hash)
      EcdsaP1363.fromDer(sign())
    }
  fun publicKeyDer() =
    store.getCertificate(alias)?.publicKey?.encoded
      ?: run {
        sign(byteArrayOf(0))
        store.getCertificate(alias).publicKey.encoded
      }
}

/**
 * SQLite-backed hash-chained journal. Backend ACK advances only a verified contiguous prefix, so
 * retention never deletes an operation that has not been durably received upstream.
 */
class AndroidTransactionJournal(
  context: Context,
  private val signer: TransactionSigner,
  private val clock: () -> Instant = { Instant.now() }
) :
  SQLiteOpenHelper(context, "beefiscal-bluecash-journal.db", null, 3),
  TransactionJournal,
  BleLocalSaleStore,
  BleUnpAllocator,
  CardTransactionStore {
  override fun onConfigure(db: SQLiteDatabase) {
    db.setForeignKeyConstraintsEnabled(true)
    db.enableWriteAheadLogging()
    db.execSQL("PRAGMA synchronous=FULL")
  }
  override fun onCreate(db: SQLiteDatabase) {
    db.execSQL(
      "CREATE TABLE journal_event(journal_seq INTEGER PRIMARY KEY AUTOINCREMENT,event_id TEXT NOT NULL UNIQUE,operation_id TEXT NOT NULL,event_type TEXT NOT NULL,occurred_at TEXT NOT NULL,payload TEXT NOT NULL,prev_hash TEXT,event_hash TEXT NOT NULL UNIQUE,signing_kid TEXT NOT NULL,signature TEXT NOT NULL,backend_ack_id TEXT,acknowledged_at TEXT)"
    )
    db.execSQL("CREATE INDEX journal_operation_idx ON journal_event(operation_id,journal_seq)")
    db.execSQL(
      "CREATE TABLE sync_checkpoint(singleton INTEGER PRIMARY KEY CHECK(singleton=1),committed_through_seq INTEGER NOT NULL,committed_event_hash TEXT,ack_id TEXT,acknowledged_at TEXT)"
    )
    db.execSQL("INSERT INTO sync_checkpoint(singleton,committed_through_seq) VALUES(1,0)")
    createBleTables(db)
    createPaymentTables(db)
  }
  override fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int) {
    if (oldVersion < 2) createBleTables(db)
    if (oldVersion < 3) createPaymentTables(db)
    require(newVersion <= 3) { "JOURNAL_MIGRATION_REQUIRED" }
  }
  private fun createPaymentTables(db: SQLiteDatabase) {
    db.execSQL(
      "CREATE TABLE IF NOT EXISTS payment_transaction(operation_id TEXT PRIMARY KEY,amount_minor INTEGER NOT NULL,rrn TEXT,authorization_code TEXT,state TEXT NOT NULL,updated_at TEXT NOT NULL)"
    )
  }
  private fun createBleTables(db: SQLiteDatabase) {
    db.execSQL(
      "CREATE TABLE IF NOT EXISTS ble_local_sale(surrogate_id TEXT PRIMARY KEY,payload TEXT NOT NULL,updated_at TEXT NOT NULL)"
    )
    db.execSQL(
      "CREATE TABLE IF NOT EXISTS ble_unp_range(singleton INTEGER PRIMARY KEY CHECK(singleton=1),prefix TEXT NOT NULL,next_sequence INTEGER NOT NULL,end_sequence INTEGER NOT NULL)"
    )
  }
  override fun find(operationId: String) =
    read(
      "SELECT * FROM journal_event WHERE operation_id=? ORDER BY journal_seq",
      arrayOf(operationId)
    )
  override fun pending(limit: Int) =
    read(
      "SELECT * FROM journal_event WHERE journal_seq>(SELECT committed_through_seq FROM sync_checkpoint WHERE singleton=1) ORDER BY journal_seq LIMIT ?",
      arrayOf(limit.coerceIn(1, 100).toString())
    )
  override fun append(operationId: String, type: String, payload: String): JournalRecord {
    val db = writableDatabase
    db.beginTransaction()
    try {
      val previous =
        db
          .rawQuery(
            "SELECT journal_seq,event_hash FROM journal_event ORDER BY journal_seq DESC LIMIT 1",
            null
          )
          .use { if (it.moveToFirst()) it.getLong(0) to it.getString(1) else 0L to null }
      val sequence = previous.first + 1
      val at = clock().toString()
      val canonical = "$sequence\n$operationId\n$type\n$at\n${previous.second?:""}\n$payload"
      val digest =
        java.security.MessageDigest.getInstance("SHA-256").digest(canonical.toByteArray())
      val hash = digest.joinToString("") { "%02x".format(it) }
      val signature = Base64.getUrlEncoder().withoutPadding().encodeToString(signer.sign(digest))
      val eventId = "$operationId-$sequence"
      db.execSQL(
        "INSERT INTO journal_event(journal_seq,event_id,operation_id,event_type,occurred_at,payload,prev_hash,event_hash,signing_kid,signature) VALUES(?,?,?,?,?,?,?,?,?,?)",
        arrayOf(
          sequence,
          eventId,
          operationId,
          type,
          at,
          payload,
          previous.second,
          hash,
          signer.keyId,
          signature
        )
      )
      db.setTransactionSuccessful()
      return JournalRecord(
        sequence,
        operationId,
        type,
        at,
        payload,
        previous.second,
        hash,
        "${signer.keyId}:$signature"
      )
    } finally {
      db.endTransaction()
    }
  }
  override fun acknowledge(throughSequence: Long, hash: String, ackId: String) {
    val db = writableDatabase
    db.beginTransaction()
    try {
      val previous =
        db
          .rawQuery("SELECT committed_through_seq FROM sync_checkpoint WHERE singleton=1", null)
          .use {
            require(it.moveToFirst())
            it.getLong(0)
          }
      require(
        throughSequence >= previous && hash.matches(Regex("[0-9a-f]{64}")) && ackId.isNotBlank()
      ) {
        "ACK_INVALID"
      }
      val exists =
        db
          .rawQuery(
            "SELECT 1 FROM journal_event WHERE journal_seq=?",
            arrayOf(throughSequence.toString())
          )
          .use { it.moveToFirst() }
      require(exists) { "ACK_SEQUENCE_UNKNOWN" }
      val now = clock().toString()
      db.execSQL(
        "UPDATE sync_checkpoint SET committed_through_seq=?,committed_event_hash=?,ack_id=?,acknowledged_at=? WHERE singleton=1",
        arrayOf(throughSequence, hash, ackId, now)
      )
      db.execSQL(
        "UPDATE journal_event SET backend_ack_id=?,acknowledged_at=? WHERE journal_seq<=? AND acknowledged_at IS NULL",
        arrayOf(ackId, now, throughSequence)
      )
      db.setTransactionSuccessful()
    } finally {
      db.endTransaction()
    }
  }
  override fun purgeBefore(cutoff: Instant): Int {
    val db = writableDatabase
    val count =
      db
        .rawQuery(
          "SELECT count(*) FROM journal_event WHERE acknowledged_at IS NOT NULL AND journal_seq<(SELECT committed_through_seq FROM sync_checkpoint WHERE singleton=1) AND occurred_at<?",
          arrayOf(cutoff.toString())
        )
        .use {
          it.moveToFirst()
          it.getInt(0)
        }
    db.execSQL(
      "DELETE FROM journal_event WHERE acknowledged_at IS NOT NULL AND journal_seq<(SELECT committed_through_seq FROM sync_checkpoint WHERE singleton=1) AND occurred_at<?",
      arrayOf(cutoff.toString())
    )
    return count
  }
  override fun checkpoint(): Pair<Long, String?> =
    readableDatabase
      .rawQuery(
        "SELECT committed_through_seq,committed_event_hash FROM sync_checkpoint WHERE singleton=1",
        null
      )
      .use {
        it.moveToFirst()
        it.getLong(0) to if (it.isNull(1)) null else it.getString(1)
      }
  override fun prepare(operationId: String, amountMinor: Long) {
    require(operationId.isNotBlank() && amountMinor > 0)
    val db = writableDatabase
    db.beginTransaction()
    try {
      val old = original(operationId)
      require(old == null || old.amountMinor == amountMinor) { "PAYMENT_ID_CONFLICT" }
      if (old == null)
        db.execSQL(
          "INSERT INTO payment_transaction(operation_id,amount_minor,state,updated_at) VALUES(?,?,?,?)",
          arrayOf(operationId, amountMinor, "PREPARED", clock().toString())
        )
      db.setTransactionSuccessful()
    } finally {
      db.endTransaction()
    }
  }
  override fun approve(value: OriginalCardTransaction) {
    writableDatabase.execSQL(
      "UPDATE payment_transaction SET rrn=?,authorization_code=?,state='APPROVED',updated_at=? WHERE operation_id=? AND amount_minor=?",
      arrayOf(
        value.rrn,
        value.authorizationCode,
        clock().toString(),
        value.operationId,
        value.amountMinor
      )
    )
  }
  override fun original(operationId: String): OriginalCardTransaction? =
    readableDatabase
      .rawQuery(
        "SELECT amount_minor,rrn,authorization_code,state FROM payment_transaction WHERE operation_id=?",
        arrayOf(operationId)
      )
      .use {
        if (!it.moveToFirst()) null
        else
          OriginalCardTransaction(
            operationId,
            it.getLong(0),
            it.getString(1) ?: "",
            it.getString(2) ?: "",
            it.getString(3)
          )
      }
  override fun markReversed(operationId: String) {
    writableDatabase.execSQL(
      "UPDATE payment_transaction SET state='REVERSED',updated_at=? WHERE operation_id=? AND state='APPROVED'",
      arrayOf(clock().toString(), operationId)
    )
  }
  override fun get(surrogateId: String): BleLocalSale? =
    readableDatabase
      .rawQuery("SELECT payload FROM ble_local_sale WHERE surrogate_id=?", arrayOf(surrogateId))
      .use { if (!it.moveToFirst()) null else decodeSale(JSONObject(it.getString(0))) }
  override fun put(sale: BleLocalSale) {
    val values =
      ContentValues().apply {
        put("surrogate_id", sale.surrogateId)
        put("payload", encodeSale(sale).toString())
        put("updated_at", clock().toString())
      }
    writableDatabase
      .insertWithOnConflict("ble_local_sale", null, values, SQLiteDatabase.CONFLICT_REPLACE)
      .also { check(it != -1L) { "BLE_SALE_PERSIST_FAILED" } }
  }
  fun provisionUnpRange(prefix: String, startSequence: Long, endSequence: Long) {
    require(prefix.isNotBlank() && startSequence in 1..endSequence) { "UNP_RANGE_INVALID" }
    val db = writableDatabase
    db.beginTransaction()
    try {
      val existing =
        db
          .rawQuery(
            "SELECT prefix,next_sequence,end_sequence FROM ble_unp_range WHERE singleton=1",
            null
          )
          .use {
            if (it.moveToFirst()) Triple(it.getString(0), it.getLong(1), it.getLong(2)) else null
          }
      require(
        existing == null ||
          (existing.first == prefix &&
            existing.second == startSequence &&
            existing.third == endSequence)
      ) {
        "UNP_RANGE_REPROVISION_CONFLICT"
      }
      db.execSQL(
        "INSERT OR IGNORE INTO ble_unp_range(singleton,prefix,next_sequence,end_sequence) VALUES(1,?,?,?)",
        arrayOf(prefix, startSequence, endSequence)
      )
      db.setTransactionSuccessful()
    } finally {
      db.endTransaction()
    }
  }
  override fun next(operatorCode: String): String {
    require(operatorCode.matches(Regex("[A-Za-z0-9]{4}"))) { "OPERATOR_CODE" }
    val db = writableDatabase
    db.beginTransaction()
    try {
      val row =
        db
          .rawQuery(
            "SELECT prefix,next_sequence,end_sequence FROM ble_unp_range WHERE singleton=1",
            null
          )
          .use {
            require(it.moveToFirst()) { "UNP_RANGE_NOT_PROVISIONED" }
            Triple(it.getString(0), it.getLong(1), it.getLong(2))
          }
      require(row.second <= row.third) { "UNP_RANGE_EXHAUSTED" }
      db.execSQL(
        "UPDATE ble_unp_range SET next_sequence=? WHERE singleton=1",
        arrayOf(row.second + 1)
      )
      db.setTransactionSuccessful()
      return "${row.first}-$operatorCode-${row.second.toString().padStart(7,'0')}"
    } finally {
      db.endTransaction()
    }
  }
  private fun encodeSale(s: BleLocalSale) =
    JSONObject()
      .put("surrogate_id", s.surrogateId)
      .put("version", s.version)
      .put("state", s.state)
      .put("unp", s.unp ?: JSONObject.NULL)
      .put(
        "payments",
        JSONArray().apply {
          s.payments.forEach { payment ->
            put(
              JSONObject()
                .put("id", payment.id)
                .put("type", payment.type)
                .put("amount", payment.amount)
            )
          }
        }
      )
      .put("fiscal_operation_id", s.fiscalOperationId ?: JSONObject.NULL)
      .put("fiscal_reference", s.fiscalReference ?: JSONObject.NULL)
      .put(
        "lines",
        JSONArray().apply {
          s.lines.forEach {
            val f = it.fiscal
            put(
              JSONObject()
                .put("line_id", it.lineId)
                .put("name", f.name)
                .put("tax_group", f.taxGroup.toString())
                .put("unit_price", f.unitPrice)
                .put("quantity", f.quantity)
                .put("discount_type", f.discountType)
                .put("discount_value", f.discountValue)
                .put("department", f.department)
                .put("unit", f.unit)
            )
          }
        }
      )
      .put(
        "processed",
        JSONObject().apply {
          s.processed.forEach { (id, r) ->
            put(
              id,
              JSONObject()
                .put("hash", r.hash)
                .put("result", r.result?.let { JSONObject(it) } ?: JSONObject.NULL)
            )
          }
        }
      )
  private fun decodeSale(v: JSONObject): BleLocalSale {
    val lines = v.getJSONArray("lines")
    val parsedLines =
      (0 until lines.length()).map {
        val x = lines.getJSONObject(it)
        BleLocalLine(
          x.getString("line_id"),
          FiscalLine(
            x.getString("name"),
            x.getString("tax_group").single(),
            x.getString("unit_price"),
            x.getString("quantity"),
            x.getInt("discount_type"),
            x.getString("discount_value"),
            x.getInt("department"),
            x.getString("unit")
          )
        )
      }
    val processed = v.getJSONObject("processed")
    val records =
      processed.keys().asSequence().associateWith { id ->
        val x = processed.getJSONObject(id)
        val result =
          if (x.isNull("result")) null
          else
            x.getJSONObject("result").keys().asSequence().associateWith {
              x.getJSONObject("result").get(it)
            }
        BleIntentRecord(x.getString("hash"), result)
      }
    val payments =
      if (v.has("payments"))
        v.getJSONArray("payments").let { array ->
          (0 until array.length()).map {
            array.getJSONObject(it).let { x ->
              SalePayment(x.getString("id"), x.getString("type"), x.getString("amount"))
            }
          }
        }
      else if (!v.has("payment") || v.isNull("payment")) emptyList()
      else
        listOf(
          v.getJSONObject("payment").let {
            SalePayment(it.getString("id"), it.getString("type"), it.getString("amount"))
          }
        )
    return BleLocalSale(
      v.getString("surrogate_id"),
      v.getLong("version"),
      v.getString("state"),
      parsedLines,
      records,
      if (v.isNull("unp")) null else v.getString("unp"),
      payments,
      v.optString("fiscal_operation_id").ifBlank { null },
      v.optString("fiscal_reference").ifBlank { null }
    )
  }
  private fun read(sql: String, args: Array<String>): List<JournalRecord> {
    val out = mutableListOf<JournalRecord>()
    readableDatabase.rawQuery(sql, args).use { c ->
      while (c.moveToNext()) out +=
        JournalRecord(
          c.getLong(c.getColumnIndexOrThrow("journal_seq")),
          c.getString(c.getColumnIndexOrThrow("operation_id")),
          c.getString(c.getColumnIndexOrThrow("event_type")),
          c.getString(c.getColumnIndexOrThrow("occurred_at")),
          c.getString(c.getColumnIndexOrThrow("payload")),
          c.getString(c.getColumnIndexOrThrow("prev_hash")),
          c.getString(c.getColumnIndexOrThrow("event_hash")),
          "${c.getString(c.getColumnIndexOrThrow("signing_kid"))}:${c.getString(c.getColumnIndexOrThrow("signature"))}",
          !c.isNull(c.getColumnIndexOrThrow("acknowledged_at"))
        )
    }
    return out
  }
}
