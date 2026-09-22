"""Generate short H.264 stress samples locally; no Plex account is involved."""
import os
import json
from pathlib import Path
import subprocess
import sys

out = Path(sys.argv[1])
out.mkdir(parents=True, exist_ok=True)
results = []
for kbps in ([int(value) for value in sys.argv[2:]] or (4500, 6000, 8000, 12000, 20000)):
    path = out / ('stress-%d.mkv' % kbps)
    subprocess.run([os.environ.get('FFMPEG', 'ffmpeg'), '-y', '-hide_banner', '-loglevel', 'error',
                    '-f', 'lavfi', '-i', 'testsrc2=size=720x480:rate=30000/1001',
                    '-f', 'lavfi', '-i', 'anullsrc=r=48000:cl=stereo', '-t', '15',
                    '-vf', 'noise=alls=35:allf=t+u:all_seed=42',
                    '-c:v', 'libx264', '-preset', 'medium', '-profile:v', 'high',
                    '-b:v', str(kbps)+'k', '-maxrate', str(kbps)+'k', '-bufsize', str(kbps*2)+'k',
                    '-g', '60', '-pix_fmt', 'yuv420p', '-c:a', 'libmp3lame', '-b:a', '192k', str(path)], check=True)
    packets = subprocess.check_output([os.environ.get('FFPROBE', 'ffprobe'), '-v', 'error', '-select_streams', 'v',
                                       '-show_entries', 'packet=size', '-of', 'csv=p=0', str(path)], text=True)
    sizes = [int(line.strip()) for line in packets.splitlines() if line.strip()]
    result = {'file': path.name, 'requested_kbps': kbps, 'video_kbps': round(sum(sizes)*8/(len(sizes)/(30000/1001))/1000),
              'frames': len(sizes), 'fps': 30000/1001}
    results.append(result)
    print(json.dumps(result), flush=True)
(out / 'samples.json').write_text(json.dumps(results, indent=2))
