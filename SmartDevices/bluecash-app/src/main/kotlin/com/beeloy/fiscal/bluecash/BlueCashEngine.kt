package com.beeloy.fiscal.bluecash

import java.security.MessageDigest
import java.time.Instant

data class SalePayment(val id:String,val type:String,val amount:String)
data class BlueCashSale(val operationId:String,val unp:String,val operator:Int,val password:String,val till:Int,val lines:List<FiscalLine>,val payments:List<SalePayment>)
data class BlueCashReversal(val operationId:String,val originalOperationId:String,val operator:Int,val password:String,val till:Int,val originalDocument:DatecsStornoDocument,val lines:List<FiscalLine>,val payments:List<SalePayment>)
data class JournalRecord(val sequence:Long,val operationId:String,val type:String,val occurredAt:String,val payload:String,val previousHash:String?,val hash:String,val signature:String, val acknowledged:Boolean=false)
interface TransactionSigner { val keyId:String; fun sign(hash:ByteArray):ByteArray }
interface TransactionJournal {
 fun find(operationId:String):List<JournalRecord>
 fun append(operationId:String,type:String,payload:String):JournalRecord
 fun pending(limit:Int=100):List<JournalRecord>
 fun acknowledge(throughSequence:Long,hash:String,ackId:String)
 fun purgeBefore(cutoff:Instant):Int
 fun checkpoint():Pair<Long,String?>
}
interface DatecsFiscalPort { fun execute(command:Int,payload:ByteArray=byteArrayOf()):DatecsResponse; fun reachable():Boolean }
interface DatecsPaymentPort { fun purchaseEur(amountMinor:Long,operationId:String):Map<String,String>; fun reverse(operationId:String):Map<String,String>; fun reachable():Boolean }

class SignedMemoryJournal(private val signer:TransactionSigner,private val clock:()->Instant={Instant.now()}):TransactionJournal {
 private val rows=mutableListOf<JournalRecord>(); private var acknowledgedThrough=0L
 override fun find(operationId:String)=rows.filter{it.operationId==operationId}
 override fun append(operationId:String,type:String,payload:String):JournalRecord { synchronized(rows){ val seq=(rows.lastOrNull()?.sequence?:0)+1; val previous=rows.lastOrNull()?.hash; val at=clock().toString(); val canonical="$seq\n$operationId\n$type\n$at\n${previous?:""}\n$payload"; val hash=MessageDigest.getInstance("SHA-256").digest(canonical.toByteArray()).joinToString(""){"%02x".format(it)}; val sig=java.util.Base64.getUrlEncoder().withoutPadding().encodeToString(signer.sign(hash.chunked(2).map{it.toInt(16).toByte()}.toByteArray())); return JournalRecord(seq,operationId,type,at,payload,previous,hash,"${signer.keyId}:$sig").also{rows+=it} } }
 override fun pending(limit:Int)=rows.filter{it.sequence>acknowledgedThrough}.take(limit)
 private var acknowledgedHash:String?=null
 override fun acknowledge(throughSequence:Long,hash:String,ackId:String){ synchronized(rows){require(throughSequence>=acknowledgedThrough&&rows.any{it.sequence==throughSequence}&&hash.matches(Regex("[0-9a-f]{64}"))&&ackId.isNotBlank());acknowledgedThrough=throughSequence;acknowledgedHash=hash} }
 override fun purgeBefore(cutoff:Instant):Int { synchronized(rows){val removable=rows.takeWhile{it.sequence<acknowledgedThrough&&Instant.parse(it.occurredAt).isBefore(cutoff)}.size;if(removable>0)rows.subList(0,removable).clear();return removable} }
 override fun checkpoint()=acknowledgedThrough to acknowledgedHash
}

class BlueCashCommandProcessor(private val fiscal:DatecsFiscalPort,private val card:DatecsPaymentPort,private val journal:TransactionJournal) {
 fun sale(command:BlueCashSale, acceptedEvidence:String="state=ACCEPTED"):Map<String,String> {
  require(command.operationId.isNotBlank()&&command.lines.isNotEmpty()&&command.payments.isNotEmpty())
  val old=journal.find(command.operationId);old.firstOrNull{it.type=="ACCEPTED"}?.let{require(it.payload==acceptedEvidence){"COMMAND_ID_PAYLOAD_CONFLICT"}};old.lastOrNull{it.type in setOf("FISCALIZED","FAILED","UNKNOWN")}?.let{return parse(it.payload)}
  if(old.any{it.type=="ACCEPTED"||it.type=="EXECUTING"})return mapOf("state" to "UNKNOWN","error_code" to "RECOVERY_REQUIRED")
  require(fiscal.reachable()){"FISCAL_DEVICE_UNREACHABLE"}; journal.append(command.operationId,"ACCEPTED",acceptedEvidence); journal.append(command.operationId,"EXECUTING","state=EXECUTING")
  val approvedCardPayments=mutableListOf<SalePayment>()
  return try {
   for(p in command.payments.filter{it.type=="CARD"}){require(card.reachable()){"PAYMENT_TERMINAL_UNREACHABLE"};journal.append(command.operationId,"PAYMENT_PREPARED","payment_id=${p.id}&amount=${p.amount}");val payment=card.purchaseEur(moneyMinor(p.amount),p.id);require(payment["approved"]=="true"){payment["error_code"]?:"CARD_DECLINED"};approvedCardPayments+=p;journal.append(command.operationId,"PAYMENT_APPROVED","payment_id=${p.id}&rrn=${payment["rrn"].orEmpty()}&authorization_code=${payment["authorization_code"].orEmpty()}")}
   ok(fiscal.execute(48,DatecsPayloads.open(command.operator,command.password,command.unp,command.till)))
   command.lines.forEach{ok(fiscal.execute(49,DatecsPayloads.line(it)))}
   command.payments.forEach{ok(fiscal.execute(53,DatecsPayloads.payment(if(it.type=="CASH")0 else 1,it.amount)))}
   val closed=ok(fiscal.execute(56)); val ref=closed.data.toString(Charsets.UTF_8).split('\t').filter{it.isNotBlank()}.lastOrNull()?:("DATECS-"+command.operationId)
   mapOf("state" to "FISCALIZED","fiscal_reference" to ref).also{journal.append(command.operationId,"FISCALIZED",encode(it))}
  } catch(e:Throwable) {
   val unknown=e.message in setOf("DATECS_EOF","DATECS_BAD_FRAME","DATECS_BCC","DATECS_CORRELATION");
   if(approvedCardPayments.isNotEmpty()){
    journal.append(command.operationId,"COMPENSATION_REQUIRED","approved_payment_ids=${approvedCardPayments.joinToString(","){it.id}}")
    val reversed=approvedCardPayments.asReversed().all{p->runCatching{card.reverse(p.id)["approved"]=="true"}.getOrDefault(false)}
    val result=if(reversed)mapOf("state" to "COMPENSATED","error_code" to (e.message?:"FISCAL_FAILURE_AFTER_CARD"))else mapOf("state" to "RECOVERY_REQUIRED","error_code" to "CARD_COMPENSATION_FAILED")
    journal.append(command.operationId,result.getValue("state"),encode(result));return result
   }
   val result=mapOf("state" to if(unknown)"UNKNOWN" else "FAILED","error_code" to (e.message?:"DATECS_FAILURE"));journal.append(command.operationId,result.getValue("state"),encode(result));result
  }
 }
 fun reverse(command:BlueCashReversal,acceptedEvidence:String="state=ACCEPTED"):Map<String,String> {
  require(command.operationId.isNotBlank()&&command.originalOperationId.isNotBlank()&&command.lines.isNotEmpty()&&command.payments.isNotEmpty())
  val old=journal.find(command.operationId);old.firstOrNull{it.type=="ACCEPTED"}?.let{require(it.payload==acceptedEvidence){"COMMAND_ID_PAYLOAD_CONFLICT"}};old.lastOrNull{it.type in setOf("REVERSED","FAILED","UNKNOWN")}?.let{return parse(it.payload)}
  if(old.any{it.type=="ACCEPTED"||it.type=="EXECUTING"})return mapOf("state" to "UNKNOWN","error_code" to "RECOVERY_REQUIRED")
  require(fiscal.reachable()){"FISCAL_DEVICE_UNREACHABLE"};journal.append(command.operationId,"ACCEPTED",acceptedEvidence);journal.append(command.operationId,"EXECUTING","state=EXECUTING")
  return try {
   for(p in command.payments.filter{it.type=="CARD"}){require(card.reachable()){"PAYMENT_TERMINAL_UNREACHABLE"};val payment=card.reverse(command.originalOperationId);require(payment["approved"]=="true"){payment["error_code"]?:"CARD_REVERSAL_DECLINED"}}
   ok(fiscal.execute(43,DatecsPayloads.stornoOpen(command.operator,command.password,command.till,command.originalDocument)))
   command.lines.forEach{ok(fiscal.execute(49,DatecsPayloads.line(it)))}
   command.payments.forEach{ok(fiscal.execute(53,DatecsPayloads.payment(if(it.type=="CASH")0 else 1,it.amount)))}
   val closed=ok(fiscal.execute(56));val ref=closed.data.toString(Charsets.UTF_8).split('\t').filter{it.isNotBlank()}.lastOrNull()?:"DATECS-${command.operationId}"
   mapOf("state" to "REVERSED","fiscal_reference" to ref).also{journal.append(command.operationId,"REVERSED",encode(it))}
  }catch(e:Throwable){val unknown=e.message in setOf("DATECS_EOF","DATECS_BAD_FRAME","DATECS_BCC","DATECS_CORRELATION");val result=mapOf("state" to if(unknown)"UNKNOWN" else "FAILED","error_code" to (e.message?:"DATECS_REVERSAL_FAILURE"));journal.append(command.operationId,result.getValue("state"),encode(result));result}
 }
 private fun ok(r:DatecsResponse):DatecsResponse { val text=r.data.toString(Charsets.UTF_8);require(text.startsWith("0\t")||text=="0"||text.isEmpty()){text.substringBefore('\t').ifBlank{"DATECS_REJECTED"}};return r }
 private fun moneyMinor(v:String):Long {require(v.matches(Regex("[0-9]+\\.[0-9]{2}")));return v.replace(".","").toLong()}
 private fun encode(v:Map<String,String>)=v.entries.sortedBy{it.key}.joinToString("&"){"${it.key}=${it.value}"}
 private fun parse(v:String)=v.split('&').associate{val p=it.split('=',limit=2);p[0] to p.getOrElse(1){""}}
}
