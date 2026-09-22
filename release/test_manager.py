import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import manager


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.card = self.base / 'card'
        self.package = self.base / 'package'
        (self.package / 'payload').mkdir(parents=True)
        self.root = self.card / 'misterzine-plex'

    def package_version(self, ident):
        files = {}
        for name in manager.PAYLOAD:
            data = (ident + name).encode()
            (self.package / 'payload' / name).write_bytes(data)
            files[name] = hashlib.sha256(data).hexdigest()
        manager.write_json(self.package / 'manifest.json', {'id': ident, 'files': files})

    @mock.patch.object(manager, 'decoder')
    def test_install_update_rollback_preserve_settings(self, decoder):
        self.package_version('alpha-1')
        manager.install(self.card, self.package)
        account = self.root / 'plexcrt.json'
        account.write_text('{"token":"TEST-ONLY-secret"}')
        key = self.root / 'beta-keys/2026-09.key'
        key.parent.mkdir()
        key.write_bytes(b'patron-test-key')
        self.package_version('alpha-2')
        manager.install(self.card, self.package)
        self.assertEqual(manager.read_state(self.root), {'current': 'alpha-2', 'previous': 'alpha-1'})
        manager.rollback(self.root)
        self.assertEqual(manager.read_state(self.root)['current'], 'alpha-1')
        manager.remove(self.card)
        self.assertTrue((self.card / 'Scripts/MisterZine-Plex-Core-Run.sh.disabled').is_file())
        self.assertEqual(account.read_text(), '{"token":"TEST-ONLY-secret"}')
        self.assertEqual(key.read_bytes(), b'patron-test-key')

    @mock.patch.object(manager, 'decoder')
    def test_rollback_to_local_pre_rename_release(self, decoder):
        self.package_version('before-rename')
        manager.install(self.card, self.package)
        previous = self.root / 'releases/before-rename'
        (previous / 'MisterZine Plex Core.rbf').rename(previous / 'MisterZine Plex.rbf')
        manifest = json.loads((previous / 'manifest.json').read_text())
        manifest['files']['MisterZine Plex.rbf'] = manifest['files'].pop('MisterZine Plex Core.rbf')
        manager.write_json(previous / 'manifest.json', manifest)
        self.package_version('after-rename')
        manager.install(self.card, self.package)
        manager.rollback(self.root)
        self.assertEqual(manager.read_state(self.root)['current'], 'before-rename')

    @mock.patch.object(manager, 'decoder')
    def test_bad_package_never_changes_active_release(self, decoder):
        self.package_version('alpha-1')
        manager.install(self.card, self.package)
        self.package_version('alpha-2')
        (self.package / 'payload/plexcrt').write_bytes(b'broken')
        with self.assertRaises(ValueError):
            manager.install(self.card, self.package)
        self.assertEqual(manager.read_state(self.root)['current'], 'alpha-1')

    @mock.patch.object(manager, 'decoder')
    def test_interrupted_activation_keeps_previous_release(self, decoder):
        self.package_version('alpha-1')
        manager.install(self.card, self.package)
        self.package_version('alpha-2')
        real_atomic = manager.atomic
        def fail_active(path, data):
            if path.name == 'active.json':
                raise OSError('simulated full card')
            real_atomic(path, data)
        with mock.patch.object(manager, 'atomic', side_effect=fail_active):
            with self.assertRaises(OSError):
                manager.install(self.card, self.package)
        self.assertEqual(manager.read_state(self.root)['current'], 'alpha-1')

    def test_core_launch_keeps_browser_at_card_root(self):
        import xml.etree.ElementTree as ET
        folder = self.root / 'releases/alpha-1'
        folder.mkdir(parents=True)
        core = folder / 'MisterZine Plex Core.rbf'
        core.write_bytes(b'core')
        entry = manager.core_launch_entry(self.root, folder)
        self.assertEqual(entry.parent, self.card)
        self.assertEqual(ET.parse(entry).findtext('rbf'),
                         'misterzine-plex/releases/alpha-1/MisterZine Plex Core')
        core.rename(folder / 'MisterZine Plex.rbf')
        manager.core_launch_entry(self.root, folder)
        self.assertEqual(ET.parse(entry).findtext('rbf'),
                         'misterzine-plex/releases/alpha-1/MisterZine Plex')

    def test_small_hdmi_framebuffer_is_enlarged_for_ring(self):
        params = self.base / 'framebuffer'
        params.mkdir()
        mode = params / 'mode'
        mode.write_text('8888 1 640 480 2560')
        manager.prepare_framebuffer(params)
        self.assertEqual(mode.read_text(), '8888 1 1920 1080 7680\n')

    def test_large_framebuffer_is_left_alone(self):
        params = self.base / 'framebuffer'
        params.mkdir()
        mode = params / 'mode'
        original = '8888 1 1920 1080 7680'
        mode.write_text(original)
        manager.prepare_framebuffer(params)
        self.assertEqual(mode.read_text(), original)

    def test_reject_path_escape(self):
        self.root.mkdir(parents=True)
        manager.write_json(self.root / 'active.json', {'current': '../elsewhere'})
        with self.assertRaises(ValueError):
            manager.read_state(self.root)

    def test_redaction(self):
        value = manager.safe_log('token=TESTsecret&v=1\nAuthorization: Bearer OTHERsecret\nhttps://private.local/path\nTESTsecret', ['TESTsecret'])
        self.assertNotIn('TESTsecret', value)
        self.assertNotIn('OTHERsecret', value)
        self.assertNotIn('private.local', value)
        self.assertIn('&v=1', value)


if __name__ == '__main__':
    unittest.main()
