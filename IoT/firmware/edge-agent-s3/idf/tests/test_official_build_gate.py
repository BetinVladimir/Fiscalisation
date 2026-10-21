"""Adversarial gate tests: unsafe inputs must fail before a firmware is accepted."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location('official_build', Path(__file__).resolve().parents[1] / 'tools/build_idf.py')
gate = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(gate)

class BuildGateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.repo = self.root / 'repo'
        self.project = self.repo / 'IoT/firmware/edge-agent-s3/idf'
        self.project.mkdir(parents=True)
        self.build = self.root / 'build'
        self.build.mkdir()
        self.sdk = self.root / 'sdk'
        self.sdk.mkdir()
        self.compiler = self.root / 'xtensa-esp32s3-elf-gcc'
        self.compiler.write_text('fixture')
        self.context = {'sdk': self.sdk, 'python': Path('python3'), 'compiler': self.compiler,
                        'tools': {'compiler': 'pinned fixture', 'cmake': 'pinned fixture', 'ninja': 'pinned fixture'}}
        self.lock = {'sdk': {'commit': 'a' * 40}, 'required_config': {'CONFIG_MBEDTLS_SSL_PROTO_TLS1_3': 'y'},
                     'gpio_definitions': {'EDGE_SD_CLK': 39, 'EDGE_SD_CMD': 38, 'EDGE_SD_D0': 40}}
        self.lockfile = self.project / 'lock.json'
        self.lockfile.write_text('{}')
        self.patches = patch.multiple(gate, PROJECT=self.project, REPO=self.repo, LOCK_FILE=self.lockfile)
        self.patches.start()
        self.addCleanup(self.patches.stop)
        (self.project / 'main').mkdir()
        (self.project / 'main/app_main.cpp').write_text('// actual fixture source\n')
        for name in ['CMakeLists.txt', 'sdkconfig.defaults', 'partitions_8mb.csv', 'dependencies.lock']:
            (self.project / name).write_text(name)
        for rel in gate.ARTIFACTS:
            file = self.build / rel
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_bytes(b'actual fixture bytes ' + rel.encode())
        (self.build / 'sdkconfig').write_text('CONFIG_MBEDTLS_SSL_PROTO_TLS1_3=y\n')
        (self.build / 'toolchain').mkdir()
        (self.build / 'toolchain/cxxflags').write_text('-DEDGE_SD_CLK=39 -DEDGE_SD_CMD=38 -DEDGE_SD_D0=40\n')
        command = (f'{self.root}/xtensa-esp32s3-elf-g++ -DIDF_VER=v5.5.6 '
                   f'@{self.build}/toolchain/cxxflags -std=gnu++2b -c app_main.cpp')
        (self.build / 'compile_commands.json').write_text(json.dumps([
            {'file': str(self.project / 'main/app_main.cpp'), 'command': command}]))
        (self.build / 'CMakeCache.txt').write_text('IDF_PATH:PATH=' + str(self.sdk))

    def proof(self):
        data = {'schema_version': 1, 'lock_sha256': gate.digest(self.lockfile), 'sources': gate.source_hashes(),
                'sdk_commit': self.lock['sdk']['commit'], 'tools': self.context['tools'],
                'artifacts': gate.inspect_outputs(self.build, self.lock, self.context)}
        (self.build / 'provenance.json').write_text(json.dumps(data))
        return data

    def verify(self):
        with patch.object(gate, 'managed_component'):
            return gate.verify(self.build, self.lock, self.context)

    def test_accepts_matching_real_files_then_rejects_tampered_firmware(self):
        self.proof()
        self.verify()
        (self.build / gate.ARTIFACTS[0]).write_bytes(b'other firmware')
        with self.assertRaisesRegex(gate.GateError, 'hash mismatch'):
            self.verify()

    def test_rejects_source_change_after_build(self):
        self.proof()
        (self.project / 'main/app_main.cpp').write_text('// changed source')
        with self.assertRaisesRegex(gate.GateError, 'sources differ'):
            self.verify()

    def test_rejects_lock_change_after_build(self):
        self.proof()
        self.lockfile.write_text('{"changed":true}')
        with self.assertRaisesRegex(gate.GateError, 'lock differs'):
            self.verify()

    def test_tls_off_cannot_be_certified_even_with_updated_artifact_hash(self):
        proof = self.proof()
        config = self.build / 'sdkconfig'
        config.write_text('# CONFIG_MBEDTLS_SSL_PROTO_TLS1_3 is not set\n')
        proof['artifacts']['sdkconfig'] = gate.digest(config)
        (self.build / 'provenance.json').write_text(json.dumps(proof))
        with self.assertRaisesRegex(gate.GateError, 'violates CONFIG_MBEDTLS_SSL_PROTO_TLS1_3'):
            self.verify()

    def test_rejects_wrong_actual_cxx_standard(self):
        f = self.build / 'compile_commands.json'
        f.write_text(f.read_text().replace('gnu++2b', 'gnu++17'))
        with self.assertRaisesRegex(gate.GateError, 'C\+\+23'):
            gate.inspect_outputs(self.build, self.lock, self.context)

    def test_rejects_board_pin_override(self):
        (self.build / 'toolchain/cxxflags').write_text('-DEDGE_SD_CLK=41 -DEDGE_SD_CMD=38 -DEDGE_SD_D0=40')
        with self.assertRaisesRegex(gate.GateError, 'EDGE_SD_CLK'):
            gate.inspect_outputs(self.build, self.lock, self.context)

    def test_rejects_symlinked_firmware(self):
        file = self.build / gate.ARTIFACTS[0]
        file.unlink()
        file.symlink_to(self.compiler)
        with self.assertRaisesRegex(gate.GateError, 'unsafe artifact'):
            gate.inspect_outputs(self.build, self.lock, self.context)

    def test_existing_artifacts_cannot_be_reused_as_fresh_success(self):
        with patch.object(gate.subprocess, 'run') as invoke:
            with self.assertRaisesRegex(gate.GateError, 'new/empty'):
                gate.build_fresh(self.build, self.lock, self.context)
            invoke.assert_not_called()

    def test_sdk_change_during_build_prevents_provenance(self):
        from types import SimpleNamespace
        output = self.root / 'new-output'
        with patch.object(gate, 'managed_component'), \
             patch.object(gate.subprocess, 'run', return_value=SimpleNamespace(returncode=0)), \
             patch.object(gate, 'preflight', side_effect=gate.GateError('SDK changed during compilation')):
            with self.assertRaisesRegex(gate.GateError, 'SDK changed'):
                gate.build_fresh(output, self.lock, self.context)
        self.assertFalse((output / 'provenance.json').exists())

    def test_wrong_sdk_rejected_before_build_invocation(self):
        subprocess.run(['git', 'init', '--quiet', str(self.sdk)], check=True)
        subprocess.run(['git', '-C', str(self.sdk), '-c', 'user.name=Fixture', '-c',
                        'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false',
                        'commit', '--quiet', '--allow-empty', '-m', 'fixture'], check=True)
        with patch.dict(os.environ, {'IDF_PATH': str(self.sdk)}, clear=True), \
             patch.object(gate, 'physical_path', side_effect=lambda p, n: Path(p)), \
             patch.object(gate, 'build_fresh') as invoke:
            with self.assertRaisesRegex(gate.GateError, 'Unsupported SDK'):
                gate.preflight(self.lock)
            invoke.assert_not_called()

    def test_environment_compiler_override_rejected_before_build(self):
        with patch.dict(os.environ, {'CC': 'unapproved-compiler'}, clear=True), \
             patch.object(gate.subprocess, 'run') as invoke:
            with self.assertRaisesRegex(gate.GateError, 'environment override: CC'):
                gate.preflight(self.lock)
            invoke.assert_not_called()

    def test_wrong_usb_hash_marker_rejected(self):
        component = self.project / 'managed_components/espressif__usb_host_cdc_acm'
        component.mkdir(parents=True)
        (component / '.component_hash').write_text('b' * 64)
        self.lock['usb_component'] = {'hash': 'c' * 64}
        with self.assertRaisesRegex(gate.GateError, 'USB component marker'):
            gate.managed_component(self.lock, self.context, True)

if __name__ == '__main__':
    unittest.main()
