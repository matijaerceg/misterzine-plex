"""Build and run the presenter's geometry and scaler checks (tools/plexfb_scale_test.c)."""
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CC = next((shutil.which(c) for c in ('cc', 'gcc', 'clang') if shutil.which(c)), None)


@unittest.skipUnless(CC and Path('/usr/include/pthread.h').exists(), 'needs a Linux C compiler')
class PresenterScaleTest(unittest.TestCase):
    def test_geometry_and_scaler(self):
        with tempfile.TemporaryDirectory() as tmp:
            exe = Path(tmp) / 'plexfb_scale_test'
            subprocess.run([CC, '-O2', '-Wall', '-Wno-unused-function', '-pthread', '-o', str(exe),
                            str(ROOT / 'tools' / 'plexfb_scale_test.c'), '-lm'], check=True)
            run = subprocess.run([str(exe)], capture_output=True, text=True)
            self.assertEqual(run.returncode, 0, run.stdout + run.stderr)
            self.assertIn(' 0 failed', run.stdout)


if __name__ == '__main__':
    unittest.main()
