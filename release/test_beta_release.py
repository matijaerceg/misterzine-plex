import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import beta_release


class BetaReleaseTests(unittest.TestCase):
    def setUp(self):
        metadata = patch('beta_release.write_build_metadata')
        metadata.start()
        self.addCleanup(metadata.stop)

    def test_key_zip_and_build(self):
        with tempfile.TemporaryDirectory() as tmp:
            private = Path(tmp)
            archive = beta_release.create_key('2026-09', private)
            key = (private / '2026-09.key').read_bytes()
            self.assertEqual(len(key), 32)
            with zipfile.ZipFile(archive) as z:
                self.assertEqual(z.read('misterzine-plex/beta-keys/2026-09.key'), key)
            with self.assertRaises(FileExistsError):
                beta_release.create_key('2026-09', private)
            self.assertEqual((private / '2026-09.key').read_bytes(), key)
            with patch('beta_release.subprocess.run') as run:
                beta_release.build('beta', '2026-09', 'go', '0.1-beta', 'test', private)
                args = run.call_args.args[0]
                self.assertIn(hashlib.sha256(key).hexdigest(), args[4])
                self.assertEqual(run.call_args.kwargs['env']['GOARCH'], 'arm')
                beta_release.build('public', '', 'go', '0.1', 'test', private)
                self.assertIn('beta.Channel=public', run.call_args.args[0][4])
                self.assertNotIn('beta.Batch=', run.call_args.args[0][4])
                self.assertNotIn('SHA256=', run.call_args.args[0][4])

    def test_code_generation_and_reuse(self):
        with tempfile.TemporaryDirectory() as tmp:
            private = Path(tmp)
            with patch('beta_release.secrets.randbelow', return_value=12345):
                path = beta_release.create_code('fixture', private)
            self.assertEqual(path.read_bytes(), b'012345')
            with self.assertRaises(FileExistsError):
                beta_release.create_code('fixture', private)
            with self.assertRaises(FileExistsError):
                beta_release.create_key('fixture', private)
            with patch('beta_release.subprocess.run') as run:
                for version in ('0.2.0-beta.1', '0.2.0-beta.2'):
                    beta_release.build('beta', 'fixture', 'go', version, version, private)
                    flags = run.call_args.args[0][4]
                    self.assertIn('beta.Channel=beta', flags)
                    self.assertIn('beta.CodeSHA256=' + hashlib.sha256(b'012345').hexdigest(), flags)
                    self.assertNotIn('012345', flags)
                for invalid in (b'12345', b'012345\n', b'abcdef'):
                    path.write_bytes(invalid)
                    with self.assertRaises(ValueError):
                        beta_release.build('beta', 'fixture', 'go', 'test', 'test', private)
            beta_release.create_key('legacy', private)
            with self.assertRaises(FileExistsError):
                beta_release.create_code('legacy', private)

    def test_invalid_configuration(self):
        with tempfile.TemporaryDirectory() as tmp:
            for batch in ('../escape', '', 'UPPER', 'has space'):
                with self.assertRaises(ValueError):
                    beta_release.create_key(batch, Path(tmp))
            with self.assertRaises(ValueError):
                beta_release.build('public', '2026-09', 'go', '0.1', 'test')


if __name__ == '__main__':
    unittest.main()
