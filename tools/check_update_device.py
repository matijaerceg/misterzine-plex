"""Run a synthetic update lifecycle on MiSTer without changing its saved install.

Upload this file and the release helpers into one temporary directory. Supply two
fixture ARM apps (public 0.0.1 and beta 0.0.2-beta.1, IDs fixture-public/fixture-beta).
The beta app uses batch fixture and synthetic code 012345. Run on the device with
--source pointing to the existing Plex root and --card pointing to a NEW directory
/media/fat/.plex-update-fixture-NAME. Stops and restores the source app. No account
data is copied. The fixture directory remains for inspection.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import sys
import zipfile

sys.path.insert(0,str(Path(__file__).resolve().parent))
import catalogue
import manager
import update_service as service


def exercise(card, source, public_app, beta_app, inspect=False):
    card, source = card.resolve(), source.resolve()
    if card.parent != Path('/media/fat') or not card.name.startswith('.plex-update-fixture-') or card.exists():
        raise ValueError('Use a new /media/fat/.plex-update-fixture-NAME directory')
    original = (source/'active.json').read_bytes()
    active = source/'releases'/manager.read_state(source)['current']
    card.mkdir()
    root=card/'misterzine-plex';root.mkdir()
    for name in ('ffmpeg','decoder.json'):
        shutil.copyfile(source/name,root/name)
    (root/'ffmpeg').chmod(0o755)
    # Match the currently confirmed output mode, without copying credentials.
    cfg=json.loads((source/'plexcrt.json').read_text())
    manager.write_json(root/'plexcrt.json',{'no_theme':True,'no_taps':True,'progressive':cfg.get('progressive',False)})
    for name in ('beta-unlocks/fixture.receipt','beta-keys/fixture.key','cache/fixture'):
        dest=root/name;dest.parent.mkdir(exist_ok=True);dest.write_bytes(b'synthetic retained data')
    work=card/'fixture-packages';work.mkdir()

    def package(ident,version,channel,app):
        folder=work/ident/'misterzine-plex-alpha';(folder/'payload').mkdir(parents=True)
        for name in manager.PAYLOAD:
            shutil.copyfile(app if name=='plexcrt' else active/name,folder/'payload'/name)
        for name in service.HELPERS: shutil.copyfile(Path(service.__file__).parent/name,folder/name)
        access={'batch':'fixture','sha256':hashlib.sha256(b'012345').hexdigest()} if channel=='beta' else None
        m={'id':ident,'version':version,'channel':channel,'access':access,
           'files':{name:manager.digest(folder/'payload'/name) for name in manager.PAYLOAD}}
        manager.write_json(folder/'manifest.json',m)
        archive=work/(ident+'.zip')
        with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED) as z:
            for path in folder.rglob('*'):
                if path.is_file():z.write(path,path.relative_to(folder.parent).as_posix())
        r=dict(id=ident,version=version,channel=channel,access=access,notes='Synthetic device fixture',
               url='https://example.org/'+archive.name,db_url='https://example.org/'+channel+'.zip',
               size=archive.stat().st_size,sha256=manager.digest(archive))
        return r,archive,folder

    def staged(archive):
        def download(card,root,release):
            target=card/catalogue.STAGING/'package.zip';target.parent.mkdir(exist_ok=True)
            shutil.copyfile(archive,target)
        return download

    a,az,ap=package('fixture-public','0.0.1','public',public_app)
    b,bz,bp=package('fixture-beta','0.0.2-beta.1','beta',beta_app)
    service.stop_manager(source)
    leave_running=False
    try:
        if inspect:
            service.prepare(card,a,staged(az));service.activate(card)
            manager.write_json(root/'updates/catalogue.json',{'schema':1,'releases':{'public':a,'beta':b}})
            staged(bz)(card,root,b)
            leave_running=True
            print('Inspection ready. Back opens Options; Updates offers the synthetic beta. Code: 012345. Stop this fixture and relaunch the source manager when finished.')
            return
        service.prepare(card,a,staged(az));service.activate(card)
        service.prepare(card,b,staged(bz))
        assert manager.read_state(root)['current']==a['id']
        service.activate(card)
        assert manager.read_state(root)['current']==b['id']
        service.stop_manager(root)
        with manager.locked(root):manager.rollback(root)
        assert service.start_and_check(root)
        # Force a genuine child startup failure, then check automatic restoration.
        bad=work/'fails';bad.write_text('#!/bin/sh\nexit 1\n');bad.chmod(0o755)
        broken,z,_=package('fixture-failed','0.0.3','public',bad)
        service.prepare(card,broken,staged(z))
        try:service.activate(card)
        except RuntimeError:pass
        else:raise AssertionError('broken app was activated')
        assert manager.read_state(root)['current']==a['id']
        service.uninstall(card,keep=True)
        for name in ('plexcrt.json','beta-unlocks/fixture.receipt','beta-keys/fixture.key','cache/fixture'):
            assert (root/name).exists(),name
        # Reinstall offline using the verified decoder copied from the source.
        for name in ('ffmpeg','decoder.json'):shutil.copyfile(source/name,root/name)
        (root/'ffmpeg').chmod(0o755)
        service.prepare(card,a,staged(az));service.activate(card)
        service.uninstall(card,keep=False)
        assert not root.exists()
        assert (source/'active.json').read_bytes()==original
        print('PASS: device install, update, startup acknowledgement, rollback, failed-start recovery, keep-data uninstall, reinstall, full uninstall')
    finally:
        if not leave_running:
            service.stop_manager(root)
            subprocess.Popen([sys.executable,str(source/'manager.py'),'run','--card',str(source.parent)],
                             stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,start_new_session=True)


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    for name in ('card','source','public-app','beta-app'):p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--inspect',action='store_true',help='Leave the fixture running for UI testing; restore the source manager afterward')
    a=p.parse_args();exercise(a.card,a.source,a.public_app,a.beta_app,a.inspect)
