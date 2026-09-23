import contextlib
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import unittest
from unittest.mock import patch
import zipfile

import build_distribution
import catalogue
import manager
import update_service as service
import publish

stop_real_manager = service.stop_manager


class ProcessTests(unittest.TestCase):
    def test_startup_requires_the_selected_bitstream(self):
        with tempfile.TemporaryDirectory() as tmp:
            proc=Path(tmp)/'12';proc.mkdir()
            (proc/'comm').write_text('MiSTer\n')
            (proc/'cmdline').write_bytes(b'MiSTer\0/media/fat/old.rbf\0')
            self.assertFalse(manager.selected_core(Path('/media/fat/new.rbf'),Path(tmp)))
            self.assertTrue(manager.selected_core(Path('/media/fat/old.rbf'),Path(tmp)))
            # The same core in a MiSTer process that predates load_core is not a switch.
            self.assertFalse(manager.selected_core(Path('/media/fat/old.rbf'),Path(tmp),before={12}))
            self.assertEqual(manager.mister_processes(Path(tmp)),{12:b'/media/fat/old.rbf'})

    def test_exited_child_does_not_block_restart(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            (root/'manager.py').write_text('import time\ntime.sleep(30)\n')
            child=subprocess.Popen([service.sys.executable,str(root/'manager.py'),'run'])
            try:
                time.sleep(.05)
                stop_real_manager(root)
                child.wait(timeout=1)
            finally:
                if child.poll() is None:child.kill();child.wait()


class UpdateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.card = Path(self.temp.name) / 'card'
        self.card.mkdir()
        self.root = self.card / 'misterzine-plex'
        self.root.mkdir()
        self.fixture = Path(self.temp.name) / 'package'
        self.fixture.mkdir()
        for target in ('decoder',):
            p = patch.object(manager, target)
            p.start(); self.addCleanup(p.stop)
        p = patch.object(service, 'stop_manager')
        p.start(); self.addCleanup(p.stop)
        p = patch.object(service, 'other_downloader', return_value=False)
        p.start(); self.addCleanup(p.stop)

    def release(self, ident='fixture-1', channel='public'):
        version = '1.0.0' if channel == 'public' else '1.1.0-beta.1'
        access = None if channel == 'public' else {'batch': 'fixture', 'sha256': hashlib.sha256(b'012345').hexdigest()}
        files = {}
        package = self.fixture / ident / 'misterzine-plex-beta'
        (package / 'payload').mkdir(parents=True)
        for name in manager.PAYLOAD:
            data = (ident + name).encode()
            (package / 'payload' / name).write_bytes(data)
            files[name] = hashlib.sha256(data).hexdigest()
        manifest = {'id': ident, 'version': version, 'channel': channel, 'access': access, 'files': files}
        (package / 'manifest.json').write_text(json.dumps(manifest))
        for name in service.HELPERS:
            (package / name).write_text('# synthetic helper ' + ident)
        archive = self.fixture / (ident + '.zip')
        with zipfile.ZipFile(archive, 'w') as z:
            for path in package.rglob('*'):
                if path.is_file():
                    z.write(path, path.relative_to(package.parent).as_posix())
        release = dict(id=ident, version=version, channel=channel, access=access, notes='Synthetic test release',
                       url='https://example.org/' + archive.name, db_url='https://example.org/' + channel + '.json.zip',
                       size=archive.stat().st_size, sha256=manager.digest(archive))
        return release, archive, package

    def deliver(self, archive):
        def download(card, root, release):
            path = card / catalogue.STAGING / 'package.zip'
            path.parent.mkdir(exist_ok=True)
            path.write_bytes(archive.read_bytes())
        return download

    def test_prepare_does_not_activate_and_restart_retains_old(self):
        old, _, package = self.release('old')
        manager.install(self.card, package)
        (self.root / 'plexcrt.json').write_text('account fixture')
        release, archive, _ = self.release('new', 'beta')
        service.prepare(self.card, release, self.deliver(archive))
        self.assertEqual(manager.read_state(self.root)['current'], 'old')
        self.assertEqual(json.loads((self.root / 'updates/status.json').read_text())['stage'], 'ready')
        service.activate(self.card, lambda root: True)
        self.assertEqual(manager.read_state(self.root), {'current': 'new', 'previous': 'old'})
        self.assertEqual((self.root / 'plexcrt.json').read_text(), 'account fixture')
        self.assertIn(release['db_url'], (self.card / 'downloader_misterzine_plex.ini').read_text())
        manager.rollback(self.root)
        self.assertEqual(manager.read_state(self.root)['current'], 'old')
        self.assertEqual((self.root / 'manager.py').read_text(), '# synthetic helper old')

    def test_failed_launch_restores_runtime_registration_and_selection(self):
        _, _, package = self.release('old')
        manager.install(self.card, package)
        original = manager.read_state(self.root)
        drop = self.card / 'downloader_misterzine_plex.ini'; drop.write_text('original registration')
        r, z, _ = self.release('new')
        service.prepare(self.card, r, self.deliver(z))
        calls = []
        def launch(root):
            calls.append(manager.read_state(root)['current'])
            return len(calls) > 1
        with self.assertRaises(RuntimeError):
            service.activate(self.card, launch)
        self.assertEqual(calls, ['new', 'old'])
        self.assertEqual(manager.read_state(self.root), original)
        self.assertEqual(drop.read_text(), 'original registration')
        self.assertEqual((self.root / 'manager.py').read_text(), '# synthetic helper old')

    def test_corrupt_download_and_interruption_preserve_current(self):
        _, _, old = self.release('old')
        manager.install(self.card, old)
        r, z, _ = self.release('new')
        z.write_bytes(b'corrupted')
        with self.assertRaises(ValueError):
            service.prepare(self.card, r, self.deliver(z))
        def fail(*args):
            raise OSError('synthetic failure')
        with self.assertRaises(OSError):
            service.prepare(self.card, r, fail)
        self.assertEqual(manager.read_state(self.root)['current'], 'old')
        # The reason reaches the status the app shows and the error log, but an
        # unexpected error is named only, never echoed.
        status = json.loads((self.root / 'updates/status.json').read_text())
        self.assertIn('Update could not be prepared: OSError.', status['message'])
        log = (self.root / service.ERROR_LOG).read_text().splitlines()
        self.assertEqual(len(log), 2)
        self.assertIn('failed verification', log[0])
        self.assertTrue(log[1].endswith('Update could not be prepared: OSError'))
        self.assertNotIn('synthetic failure', log[1])

    def test_external_staging_is_reused_without_running_downloader(self):
        r, z, _ = self.release()
        self.deliver(z)(self.card, self.root, r)
        service.prepare(self.card, r, lambda *args: self.fail('Downloader should not run'))
        self.assertFalse((self.root / 'active.json').exists())

    def test_unsafe_archives_and_identity_mismatch(self):
        for name in ('../escape', '/absolute', 'misterzine-plex-beta/../../escape', 'C:/escape'):
            with self.subTest(name=name):
                r, z, _ = self.release('unsafe-' + str(abs(hash(name))))
                with zipfile.ZipFile(z, 'a') as archive:
                    archive.writestr(name, 'bad')
                r.update(size=z.stat().st_size, sha256=manager.digest(z))
                with tempfile.TemporaryDirectory() as target, self.assertRaises(ValueError):
                    service.unpack(z, Path(target), r)
        r, z, _ = self.release('mismatch')
        r['id'] = 'different'
        with tempfile.TemporaryDirectory() as target, self.assertRaises(ValueError):
            service.unpack(z, Path(target), r)

    def test_uninstall_keeps_data_and_does_not_touch_other_apps(self):
        _, _, package = self.release()
        manager.install(self.card, package)
        (self.card / 'downloader_misterzine_plex.ini').write_text('fixture')
        unrelated = self.card / 'downloader.ini'; unrelated.write_text('[other]\n')
        other_app = self.card / 'misterzine'; other_app.mkdir(); (other_app / 'keep').touch()
        for name in ('plexcrt.json', 'cache/art', 'beta-unlocks/fixture.receipt', 'beta-keys/fixture.key'):
            path = self.root / name; path.parent.mkdir(exist_ok=True); path.write_text('keep')
        service.uninstall(self.card)
        self.assertFalse((self.root / 'releases').exists())
        self.assertFalse((self.root / 'active.json').exists())
        self.assertFalse((self.card / 'downloader_misterzine_plex.ini').exists())
        self.assertEqual((self.root / 'beta-unlocks/fixture.receipt').read_text(), 'keep')
        self.assertEqual(unrelated.read_text(), '[other]\n')
        self.assertTrue((other_app / 'keep').exists())
        service.uninstall(self.card, keep=False)
        self.assertFalse(self.root.exists())

    def test_downloader_engines_and_isolated_environment(self):
        scripts = self.card / 'Scripts'; scripts.mkdir()
        base = scripts / '.config/downloader'; base.mkdir(parents=True)
        with self.assertRaises(RuntimeError):
            service.engine(self.card, bootstrap=False)
        archive = base / 'downloader_latest.zip'; archive.touch()
        self.assertEqual(service.engine(self.card)[-1], str(archive))
        if os.name != 'nt':
            binary = base / 'downloader_bin'; binary.write_text(''); binary.chmod(0o755)
            self.assertEqual(service.engine(self.card), [str(binary)])
        launcher = scripts / 'downloader.sh'; launcher.touch()
        self.assertEqual(service.engine(self.card), ['/bin/bash', str(launcher)])
        r, _, _ = self.release()
        (self.root / 'updates').mkdir()
        def run(command, **kw):
            self.assertEqual(command[-2:], ['--run-only', 'misterzine_plex'])
            env = kw['env']
            self.assertEqual(env['UPDATE_LINUX'], 'false')
            self.assertEqual(env['ALLOW_REBOOT'], '0')
            self.assertEqual(env['EXTRA_DROP_IN_DATABASE_FILES'], '')
            ini = Path(env['DOWNLOADER_INI_PATH']).read_text()
            self.assertIn('filter =', ini)
            self.assertNotIn('[distribution_mister]', ini)
            return subprocess.CompletedProcess(command, 0)
        service.download(self.card, self.root, r, run)

    def test_certificate_option_fits_downloader_limit(self):
        # Downloader refuses CURL_SSL over 50 characters, so the long bundle path
        # under Scripts must never be passed as an option.
        scripts = self.card / 'Scripts/.config/downloader'; scripts.mkdir(parents=True)
        own = scripts / 'cacert.pem'; own.write_text('')
        missing = self.card / 'no-system-bundle.pem'
        env = service.certificate_env(self.card, system=missing)
        self.assertEqual(env['SSL_CERT_FILE'], str(own))
        self.assertNotIn('CURL_SSL', env)
        system = self.card / 'cacert.pem'; system.write_text('')
        env = service.certificate_env(self.card, system=system)
        self.assertEqual(env['SSL_CERT_FILE'], str(system))
        option = '--cacert ' + str(system)
        self.assertEqual(env.get('CURL_SSL'), option if len(option) <= service.CURL_SSL_MAX else None)

        self.assertLessEqual(len('--cacert /etc/ssl/certs/cacert.pem'), service.CURL_SSL_MAX)

    def opener(self, data):
        class Response(io.BytesIO):
            def geturl(self):
                return 'https://objects.example.org/redirected/package.zip'
        return lambda url, timeout: Response(data)

    def failing_runner(self, output):
        def run(command, **kw):
            kw['stdout'].write(output)
            return subprocess.CompletedProcess(command, 1)
        return run

    def test_failed_downloader_run_falls_back_to_a_direct_fetch(self):
        r, z, _ = self.release()
        (self.card / 'Scripts').mkdir(); (self.card / 'Scripts/downloader.sh').touch()
        (self.root / 'updates').mkdir()
        output = b'curl: (60) SSL certificate problem: certificate is not yet valid\nhttps://secret.example.org/x\n'
        downloader = lambda card, root, release: service.download(
            card, root, release, self.failing_runner(output), lambda *a: service.fetch_release(*a, opener=self.opener(z.read_bytes())))
        service.prepare(self.card, r, downloader)
        staged = self.card / catalogue.STAGING / 'package.zip'
        self.assertEqual(manager.digest(staged), r['sha256'])
        self.assertFalse((self.root / 'updates/package.part').exists())
        status = json.loads((self.root / 'updates/status.json').read_text())
        self.assertEqual(status['stage'], 'ready')
        log = (self.root / 'updates/download-output.log').read_text()
        self.assertIn('Downloader could not verify the server certificate (exit 1). The release was then fetched directly.', log)

    def test_both_download_paths_failing_name_fixed_reasons_only(self):
        r, _, _ = self.release()
        (self.card / 'Scripts').mkdir(); (self.card / 'Scripts/downloader.sh').touch()
        (self.root / 'updates').mkdir()
        output = b'downloader.ini: CURL_SSL value too long https://secret.example.org/cert\n'
        def refused(url, timeout):
            raise service.urllib.error.HTTPError(url, 404, 'Not Found', {}, None)
        # GitHub answers, so the screen blames the release; the log keeps both reasons.
        with self.assertRaises(RuntimeError):
            service.prepare(self.card, r, lambda card, root, release: service.download(
                card, root, release, self.failing_runner(output), lambda *a: service.fetch_release(*a, opener=refused), lambda: None))
        status = json.loads((self.root / 'updates/status.json').read_text())
        self.assertIn('Download failed: GitHub is reachable, so the release itself is at fault. Please report this.', status['message'])
        self.assertNotIn('Downloader', status['message'])
        log = (self.root / service.ERROR_LOG).read_text()
        self.assertIn('[Downloader rejected its certificate option (exit 1); '
                      'direct download was refused the release file (HTTP 404)]', log)
        self.assertNotIn('example.org', status['message'] + log)
        # A missing Downloader that cannot be bootstrapped is reported the same
        # way, and with no DNS the screen blames the connection.
        (self.card / 'Scripts/downloader.sh').unlink()
        def offline(url, timeout):
            raise service.urllib.error.URLError(service.socket.gaierror(-2, 'Name or service not known'))
        with patch.object(service.urllib.request, 'urlopen', offline), self.assertRaises(RuntimeError) as caught:
            service.download(self.card, self.root, r, self.failing_runner(b''), lambda *a: service.fetch_release(*a, opener=offline))
        self.assertEqual(str(caught.exception), 'Download failed: your MiSTer cannot look up GitHub. Check its network connection and DNS.')
        self.assertEqual(caught.exception.detail, 'Downloader setup could not resolve the download host; '
                         'direct download could not resolve the download host')

    def test_verdict_separates_the_board_from_the_release(self):
        e = service.urllib.error
        def probe(exc):
            def opener(request, timeout):
                self.assertEqual(request.get_method(), 'HEAD')
                raise exc
            return opener
        self.assertIn('cannot look up GitHub', service.verdict(probe(e.URLError(service.socket.gaierror(-2, 'x')))))
        self.assertIn('cannot verify GitHub certificates', service.verdict(probe(e.URLError('[SSL: CERTIFICATE_VERIFY_FAILED]'))))
        self.assertIn('cannot reach GitHub', service.verdict(probe(e.URLError(service.socket.timeout()))))
        self.assertIn('cannot reach GitHub', service.verdict(probe(ConnectionResetError())))
        self.assertIsNone(service.verdict(probe(e.HTTPError('https://github.com/', 403, 'Forbidden', {}, None))))
        self.assertIsNone(service.verdict(self.opener(b'')))
        # A clock from before this code existed explains certificate failures without a probe.
        self.assertIn('clock reads 2016', service.verdict(lambda *a: self.fail('no probe'), now=lambda: 1461000000))

    def test_direct_fetch_verifies_size_and_digest_and_leaves_no_partial_file(self):
        r, z, _ = self.release()
        (self.root / 'updates').mkdir()
        for data in (z.read_bytes() + b'x', z.read_bytes()[:-1], b'y' * r['size']):
            with self.assertRaises(ValueError) as caught:
                service.fetch_release(self.card, self.root, r, self.opener(data))
            self.assertIn('release file', str(caught.exception))
        self.assertFalse((self.card / catalogue.STAGING / 'package.zip').exists())
        self.assertFalse((self.root / 'updates/package.part').exists())
        def plain_http(url, timeout):
            class Response(io.BytesIO):
                def geturl(self):
                    return 'http://example.org/package.zip'
            return Response(z.read_bytes())
        with self.assertRaises(ValueError):
            service.fetch_release(self.card, self.root, r, plain_http)
        service.fetch_release(self.card, self.root, r, self.opener(z.read_bytes()))
        self.assertEqual(manager.digest(self.card / catalogue.STAGING / 'package.zip'), r['sha256'])

    def test_failure_reasons_are_fixed_words(self):
        log = self.root / 'output.log'
        for text, expected in ((b'Could not resolve host: raw.githubusercontent.com', 'could not resolve the download host (exit 1)'),
                               (b'OSError: [Errno 28] No space left on device', 'found no free space (exit 1)'),
                               (b'Traceback (most recent call last):', 'crashed (exit 1)'),
                               (b'Bad status code 503', 'got a server error (exit 1)'),
                               (b'something else entirely', 'exit 1, see updates/download-output.log')):
            log.write_bytes(text)
            self.assertEqual(service.downloader_reason(log, 1), expected)
        self.assertEqual(service.downloader_reason(self.root / 'missing.log', 2), 'exit 2')
        e = service.urllib.error
        self.assertEqual(service.network_reason(e.URLError(service.socket.timeout('timed out'))), 'timed out')
        self.assertEqual(service.network_reason(e.URLError(ConnectionRefusedError())), 'could not connect')
        self.assertEqual(service.network_reason(e.URLError(OSError(101, 'Network is unreachable'))), 'failed with network is unreachable')
        self.assertEqual(service.network_reason(e.URLError('[SSL: CERTIFICATE_VERIFY_FAILED] certificate verify failed')), 'could not verify the server certificate')
        self.assertEqual(service.network_reason(ValueError('Downloader archive verification failed')), 'Downloader archive verification failed')

    def test_diagnostics_include_download_logs_without_addresses(self):
        (self.root / 'updates').mkdir()
        (self.root / 'updates/download-output.log').write_text('first\ncurl: (6) Could not resolve host https://raw.example.org/db.json.zip\n')
        (self.root / 'updates/downloader.log').write_text('first\nDownloader 2.0 https://example.org/x\n')
        with contextlib.redirect_stdout(io.StringIO()):
            manager.diagnostics(self.root)
        report = json.loads((self.root / 'diagnostics.json').read_text())
        self.assertIn('Could not resolve host [server address removed]', report['logs']['download-output.log'])
        self.assertIn('Downloader 2.0 [server address removed]', report['logs']['downloader.log'])

    def test_publishing_database_owns_only_staging_and_installer_is_standalone(self):
        r, z, _ = self.release()
        out = self.fixture / 'dist'
        published = build_distribution.build(z, 'v1.0.0', 'Release notes', out)
        catalogue.entry(published)
        with zipfile.ZipFile(out / 'public.json.zip') as db:
            data = json.loads(db.read('misterzine_plex.json'))
            self.assertEqual(set(data['files']), {'misterzine-plex-downloads/package.zip'})
        script = (out / 'MisterZine-Plex-Install-Public.sh').read_text()
        self.assertIn('service.pyz', script)
        self.assertNotIn('update_all', script)
        self.assertNotIn('012345', script)
        self.assertIn('--channel public', script)
        self.assertIn('--yes', script)
        self.assertFalse((out / 'MisterZine-Plex-Install-Beta.sh').exists())
        pinned = (out / 'MisterZine-Plex-Install-1.0.0.sh').read_text()
        self.assertIn(published['sha256'], pinned)
        self.assertIn('--request', pinned)
        self.assertIn('/v1.0.0/release-db.json.zip', pinned)
        self.assertEqual((out/'release-db.json.zip').read_bytes(), (out/'public.json.zip').read_bytes())

    def test_menu_entry_repair_and_uninstall_preserve_other_startup_commands(self):
        startup = self.card / 'linux/user-startup.sh'
        startup.parent.mkdir()
        startup.write_text('#!/bin/bash\necho other-app\nexit 0\n')
        _, _, package = self.release()
        manager.install(self.card, package)
        manager.install(self.card, package)
        self.assertTrue((self.root/'updates').is_dir())
        text = startup.read_text()
        self.assertEqual(text.count(manager.BOOT_START), 1)
        self.assertLess(text.index(manager.BOOT_START), text.index('exit 0'))
        self.assertTrue((self.card/'MisterZine Plex Core.mgl').exists())
        self.assertFalse((self.card/'Scripts/MisterZine-Plex-Run.sh').exists())
        service.uninstall(self.card)
        self.assertEqual(startup.read_text(), '#!/bin/bash\necho other-app\nexit 0\n')
        self.assertFalse((self.card/'MisterZine Plex Core.mgl').exists())

    def test_unattended_pinned_install_does_not_fetch_latest_or_prompt(self):
        release, _, _ = self.release()
        request = self.fixture/'request.json'
        request.write_text(json.dumps(release))
        with patch('sys.argv', ['worker','install','--card',str(self.card),'--request',str(request),'--yes',
                                '--download-db-url','https://example.org/immutable.json.zip']), \
             patch.object(service.platform,'system',return_value='Linux'), \
             patch.object(service.platform,'machine',return_value='armv7l'), \
             patch.object(Path,'exists',return_value=True), \
             patch('builtins.input',side_effect=AssertionError('unexpected prompt')), \
             patch.object(catalogue,'fetch',side_effect=AssertionError('unexpected latest lookup')), \
             patch.object(service,'prepare') as prepare, patch.object(service,'activate'), \
             patch.object(service,'download') as download:
            service.main()
            self.assertEqual(prepare.call_args.args[1], release)
            prepare.call_args.args[2](self.card,self.root,release)
            self.assertEqual(download.call_args.args[2]['db_url'],'https://example.org/immutable.json.zip')
            self.assertEqual(release['db_url'],'https://example.org/public.json.zip')

    def test_metadata_rejects_unknown_channels_and_secrets(self):
        r, _, _ = self.release()
        catalogue.catalogue({'schema': 1, 'releases': {'public': r}})
        for field, value in [('url', 'http://example.org/file'), ('id', '../escape'), ('size', -1), ('sha256', 'bad'), ('channel', 'other')]:
            with self.subTest(field=field), self.assertRaises(ValueError):
                catalogue.entry(dict(r, **{field: value}))
        with self.assertRaises(ValueError):
            catalogue.entry(dict(r, access={'code': '012345'}))

    def test_tampered_helpers_and_worker_exclusion(self):
        r, archive, _ = self.release()
        service.prepare(self.card, r, self.deliver(archive))
        (self.root / 'updates/ready/manager.py').write_text('altered')
        with self.assertRaises(ValueError):
            service.activate(self.card, launch=lambda root: True)
        with service.worker_lock(self.root), self.assertRaises(RuntimeError):
            service.prepare(self.card, r, self.deliver(archive))

    def test_bootstrap_rejects_unverified_download(self):
        class Response(io.BytesIO):
            def geturl(self): return service.BOOTSTRAP_URL
        with patch.object(service.urllib.request, 'urlopen', return_value=Response(b'corrupt')), self.assertRaises(ValueError):
            service.engine(self.card)
        self.assertFalse((self.card / 'Scripts/.config/downloader/downloader_latest.zip').exists())

    def test_interrupted_activation_restores_selection_and_helpers(self):
        _, _, old = self.release('old')
        manager.install(self.card, old)
        recovery = self.root / 'updates/recovery'; recovery.mkdir(parents=True)
        manager.write_json(recovery/'selection.json', manager.read_state(self.root))
        (recovery/'registration').write_bytes(b'[misterzine_plex]\n')
        (recovery/'manager.py').write_bytes((self.root/'manager.py').read_bytes())
        manager.write_json(self.root/'updates/activation.json', {'helpers':['manager.py']})
        manager.write_json(self.root/'active.json', {'current':'failed','previous':'old'})
        (self.root/'manager.py').write_text('new helper')
        with patch.dict(os.environ, {}, clear=True):
            self.assertTrue(manager.recover_activation(self.root))
        self.assertEqual(manager.read_state(self.root)['current'],'old')
        self.assertEqual((self.root/'manager.py').read_bytes(),(old/'manager.py').read_bytes())
        self.assertFalse((self.root/'updates/activation.json').exists())

    def test_publishing_defaults_to_local_preparation(self):
        _, archive, _ = self.release()
        with patch.object(publish, 'run', side_effect=AssertionError('unexpected remote action')):
            publish.publish(archive,'v1.0.0','Synthetic notes',self.fixture/'publish')
