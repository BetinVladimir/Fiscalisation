import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
STORAGE = (ROOT / "main" / "durable_storage.cpp").read_text()
PROCESSOR = (ROOT / "main" / "intent_processor.cpp").read_text()


class AtomicReservationContract(unittest.TestCase):
    def test_command_reservation_owns_one_immediate_transaction(self):
        start = STORAGE.index("AtomicReservationResult DurableStorage::reserve_command")
        end = STORAGE.index("esp_err_t DurableStorage::set_operation_state", start)
        body = STORAGE[start:end]
        self.assertEqual(body.count('BEGIN IMMEDIATE;'), 1)
        self.assertIn("INSERT INTO operations", body)
        self.assertIn("UPDATE authority_state", body)
        self.assertIn("INSERT INTO receipts", body)
        self.assertIn("INSERT INTO payments", body)
        self.assertIn("INSERT INTO event_outbox", body)
        self.assertEqual(body.count('COMMIT;'), 2)  # duplicate read and new reservation

    def test_processor_does_not_split_authority_or_accepted_event(self):
        self.assertIn("storage_.reserve_command(", PROCESSOR)
        self.assertNotIn("storage_.reserve_authority(", PROCESSOR)
        execute = PROCESSOR[PROCESSOR.index("esp_err_t IntentProcessor::execute"):PROCESSOR.index("esp_err_t IntentProcessor::accept")]
        self.assertNotIn('storage_.append_event(event.c_str()', execute)

    def test_storage_pressure_fails_before_reservation_and_physical_io(self):
        execute = PROCESSOR[PROCESSOR.index("esp_err_t IntentProcessor::execute"):PROCESSOR.index("esp_err_t IntentProcessor::accept")]
        self.assertLess(execute.index("can_accept_operation"), execute.index("reserve_command"))
        self.assertLess(execute.index("can_accept_operation"), execute.index("executor_.execute"))
        self.assertIn("esp_vfs_fat_info", STORAGE)
        self.assertIn("free_bytes", STORAGE)


if __name__ == "__main__":
    unittest.main()
