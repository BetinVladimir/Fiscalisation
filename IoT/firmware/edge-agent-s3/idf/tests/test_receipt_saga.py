import pathlib,unittest
ROOT=pathlib.Path(__file__).resolve().parents[1]
SRC=(ROOT/'main'/'receipt_saga.cpp').read_text()
class SagaContract(unittest.TestCase):
 def test_card_precedes_fiscal_and_failure_reverses(self):
  self.assertLess(SRC.index('payment_->purchase'),SRC.index('fiscal_.fiscalize'))
  self.assertGreater(SRC.index('payment_->reverse'),SRC.index('fiscal_.fiscalize'))
 def test_unknown_is_never_blindly_retried(self):
  self.assertIn('PaymentCertainty::Unknown',SRC);self.assertIn('fiscal_.lookup',SRC)
  self.assertNotIn('for(int retry',SRC)
if __name__=='__main__':unittest.main()
