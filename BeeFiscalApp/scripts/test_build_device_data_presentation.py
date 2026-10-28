import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('builder', Path(__file__).with_name('build-device-data-presentation.py'))
builder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(builder)

class PresentationBuildTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / 'Fiscalisation/BeeFiscalApp'
        self.root.mkdir(parents=True)
        for name in builder.FILES:
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(json.dumps({'main': 'production.js'}) if name == 'package.json' else '{}')
        (self.root / 'src').mkdir()
        (self.root / 'src/example.ts').write_text('export const example=1;')
        (self.root / 'store/credentials.local.json').write_text('PRIVATE_DO_NOT_COPY')
        (self.root / 'node_modules').mkdir()
        for name in ('design-tokens', 'shared-ui'):
            directory = self.base / 'BeeloyBackend/mobile' / name
            directory.mkdir(parents=True)
            (directory / 'index.js').write_text('module.exports={};')
        self.output = self.base / 'published'
        self.output.mkdir()
        (self.output / 'old').write_text('preserve')

    def run_export(self, command, cwd, env, **kwargs):
        app = Path(cwd)
        self.assertEqual(json.loads((app / 'package.json').read_text())['main'], builder.ENTRYPOINT)
        self.assertFalse((app / 'store/credentials.local.json').exists())
        self.assertEqual(env['EXPO_NO_DOTENV'], '1')
        self.assertNotIn('EXPO_PUBLIC_PRIVATE_TEST', env)
        target = Path(command[-1])
        target.mkdir()
        (target / 'index.html').write_text('synthetic')
        return subprocess.CompletedProcess(command, 0)

    def test_isolated_export_manifest_and_replacement(self):
        original = (self.root / 'package.json').read_bytes()
        with patch.object(builder.subprocess, 'run', side_effect=self.run_export), patch.dict(builder.os.environ, {'EXPO_PUBLIC_PRIVATE_TEST': 'secret'}):
            manifest = builder.build(self.root, self.output)
        self.assertEqual((self.root / 'package.json').read_bytes(), original)
        self.assertFalse((self.output / 'old').exists())
        self.assertEqual(manifest['kind'], builder.KIND)
        self.assertEqual(manifest['export_sha256'], {'index.html': builder.hashlib.sha256(b'synthetic').hexdigest()})
        self.assertEqual(json.loads((self.output / builder.MANIFEST).read_text()), manifest)
        self.assertFalse(list(self.base.glob('.presentation-build-*')))

    def test_failed_export_preserves_previous_output(self):
        with patch.object(builder.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, 'expo')):
            with self.assertRaises(subprocess.CalledProcessError):
                builder.build(self.root, self.output)
        self.assertEqual((self.output / 'old').read_text(), 'preserve')
        self.assertFalse(list(self.base.glob('.presentation-build-*')))

    def test_publish_failure_restores_previous_output(self):
        rename = Path.rename
        def fail_publish(path, target):
            if path.name == 'export':
                raise OSError('simulated publication failure')
            return rename(path, target)
        with patch.object(builder.subprocess, 'run', side_effect=self.run_export), patch.object(Path, 'rename', fail_publish):
            with self.assertRaises(OSError):
                builder.build(self.root, self.output)
        self.assertEqual((self.output / 'old').read_text(), 'preserve')
        self.assertFalse(list(self.base.glob('.presentation-backup-*')))

    def test_missing_index_and_symlinked_artifact_are_rejected(self):
        for mode in ('missing-index', 'symlink'):
            def invalid_export(command, **kwargs):
                target = Path(command[-1])
                target.mkdir()
                if mode == 'symlink':
                    (target / 'index.html').symlink_to(self.root / 'app.json')
            with patch.object(builder.subprocess, 'run', side_effect=invalid_export):
                with self.assertRaises(ValueError):
                    builder.build(self.root, self.output)
            self.assertEqual((self.output / 'old').read_text(), 'preserve')

    def test_output_guard_and_missing_input(self):
        for output in (Path('relative'), self.root, self.root / 'export', self.base):
            with self.assertRaises(ValueError):
                builder.build(self.root, output)
        (self.root / 'e2e/device-data-fixtures.ts').unlink()
        with self.assertRaises(ValueError):
            builder.build(self.root, self.output)
        self.assertEqual((self.output / 'old').read_text(), 'preserve')

    def test_shared_source_drift_preserves_previous_export(self):
        def change_shared(*args, **kwargs):
            result = self.run_export(*args, **kwargs)
            (self.base / 'BeeloyBackend/mobile/shared-ui/index.js').write_text('changed during build')
            return result
        with patch.object(builder.subprocess, 'run', side_effect=change_shared):
            with self.assertRaisesRegex(ValueError, 'Shared source changed'):
                builder.build(self.root, self.output)
        self.assertEqual((self.output / 'old').read_text(), 'preserve')

if __name__ == '__main__':
    unittest.main()
