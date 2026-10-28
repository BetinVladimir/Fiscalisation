"""Wiring checks complement execution of the actual guard/identity in native tests."""
import pathlib
import unittest
ROOT = pathlib.Path(__file__).resolve().parents[1]
class StartupEntropySource(unittest.TestCase):
    def test_identity_guard_precedes_all_radio_and_profile_startup(self):
        source=(ROOT/'main/app_main.cpp').read_text().split('extern "C" void app_main(void)',1)[1]
        guarded=source.index('with_startup_entropy([]{return device_identity.open();})')
        for call in ['ble_provisioning_start(', 'provisioned_network_start(', 'physical_io.begin(', 'mqtt_runtime_start(', 'sync_runtime_start(', 'ble_runtime_start(']:
            self.assertLess(guarded,source.index(call),call)
        self.assertEqual(source.count('device_identity.open()'),1)
    def test_guard_dependency_is_explicit_and_failure_cleans_up(self):
        source=(ROOT/'main/CMakeLists.txt').read_text()
        self.assertIn('REQUIRES bootloader_support ',source)
        guard=(ROOT/'main/startup_entropy.h').read_text()
        self.assertIn('~Scope(){bootloader_random_disable();}',guard)
    def test_device_rng_callback_never_reads_unconditioned_hardware(self):
        source=(ROOT/'main/device_identity.cpp').read_text()
        self.assertEqual(source.count('esp_fill_random('),1)
        self.assertIn('mbedtls_ctr_drbg_seed(&impl_->rng,hardware_entropy',source)
        self.assertNotIn('random,nullptr',source)
        self.assertIn('mbedtls_ctr_drbg_set_reseed_interval(&impl_->rng,INT_MAX)',source)
        self.assertIn('maximum_random_requests',source)
if __name__=='__main__':unittest.main()
