import io
import tempfile
import unittest
import zipfile
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import patch

import build_package


class PackagePrivacyTests(unittest.TestCase):
    def test_private_access_files_never_enter_package(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            files = ['app/dist/plexcrt', 'arm/plexplay.py', 'arm/plexfb',
                     'arm/plexfb.c', 'core/PlexCRT.rbf', 'core/PlexCRT.qsf',
                     'core/PlexCRT.qpf', 'core/PlexCRT.sdc', 'core/build_id.v',
                     'core/LICENSE', 'core/PlexCRT.sv', 'core/files.qip',
                     'core/LICENSING.md', 'core/rtl/fixture.v', 'core/sys/fixture.v',
                     'release/manager.py', 'release/update_service.py', 'release/catalogue.py', 'release/menu_launcher.py', 'release/README.md', 'release/TERMS.md',
                     'release/THIRD_PARTY_NOTICES.md', 'release/BETA_ACCESS.md',
                     'release/licenses/fixture.txt']
            for name in files:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(b'synthetic public fixture')
            for folder in ('release/private-beta', 'app/dist', 'core/sys'):
                for suffix in ('.key', '.code', '.receipt'):
                    path = root / folder / ('private-fixture' + suffix)
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(b'SYNTHETIC-PRIVATE-MATERIAL')
            with patch.object(build_package, 'ROOT', root), redirect_stdout(io.StringIO()):
                build_package.build(root / 'out', root / 'core', 'fixture', '0.2.0-beta.1')
            with zipfile.ZipFile(next((root / 'out').glob('*.zip'))) as archive:
                for name in archive.namelist():
                    self.assertFalse(name.endswith(('.key', '.code', '.receipt')))
                    data = archive.read(name)
                    self.assertNotIn(b'SYNTHETIC-PRIVATE-MATERIAL', data)
                    if name.endswith('corresponding-source.zip'):
                        with zipfile.ZipFile(io.BytesIO(data)) as source:
                            for member in source.namelist():
                                self.assertFalse(member.endswith(('.key', '.code', '.receipt')))
                                self.assertNotIn(b'SYNTHETIC-PRIVATE-MATERIAL', source.read(member))
