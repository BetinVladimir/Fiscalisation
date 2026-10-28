package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

class AggregateRebuildTest {
  @Test
  fun aggregateRebuildMatchesJournal() {
    val sale1 = BleLocalSale("s1", 1L, "OPEN", emptyList(), emptyMap())
    val sale2 = BleLocalSale("s2", 1L, "FISCALIZED", emptyList(), mapOf("i1" to BleIntentRecord("hash1", mapOf("status" to "FISCALIZED"))))
    val source = MemoryBleLocalSaleStore().also { it.put(sale1); it.put(sale2) }
    val target = MemoryBleLocalSaleStore()

    rebuildAggregateFromJournal(source, target)

    assertEquals(sale1, target.get("s1"))
    assertEquals(sale2, target.get("s2"))
    assertEquals(null, target.get("s3"))
  }

  @Test
  fun aggregateRebuildIsIdempotent() {
    val sale = BleLocalSale("s1", 2L, "FISCALIZED", emptyList(), emptyMap())
    val source = MemoryBleLocalSaleStore().also { it.put(sale) }
    val target = MemoryBleLocalSaleStore()

    rebuildAggregateFromJournal(source, target)
    rebuildAggregateFromJournal(source, target)

    assertEquals(sale, target.get("s1"))
    assertEquals(1, target.listAll().size)
  }

  @Test
  fun emptyJournalProducesEmptyAggregate() {
    val source = MemoryBleLocalSaleStore()
    val target = MemoryBleLocalSaleStore()
    rebuildAggregateFromJournal(source, target)
    assertEquals(0, target.listAll().size)
  }
}
