"""Turn the MiSTerZine autotrace into editable vector source.

Input is brand/misterzine-source-trace.svg, vendored verbatim from the zine repo
(logo_white.svg, md5 3a99a0ae) so this repo doesn't reach into a sibling working
tree. It is a potrace output: three <path>s under a `translate(0,600)
scale(0.1,-0.1)` group, where the whole two-line wordmark is a single compound
path whose letters are HOLES in the sticker silhouette. Good for rasterizing
onto a ground, useless for design work.

This splits it into named, independently fillable objects and writes:

  brand/misterzine-knockout.svg  faithful original: one path, letters knocked out
  brand/misterzine-layered.svg   Sticker + per-glyph paths, two-tone, Affinity-ready

Both get the group transform baked in (no negative scale -- Affinity treats a
flipped object oddly under boolean ops) and a viewBox cropped to the ink.

Run from the repository root: python brand/tools/prep_wordmark.py
Needs svgpathtools.
"""

import re
import xml.etree.ElementTree as ET
from pathlib import Path

from svgpathtools import parse_path

OUT = Path(__file__).resolve().parents[1]
SRC = OUT / "misterzine-source-trace.svg"

SVG_NS = "{http://www.w3.org/2000/svg}"

# Ground and ink of the on-device startup logo, so the layered file opens
# looking like the thing it came from rather than as two black rectangles.
STICKER = "#1d1330"
LETTERS = "#a293c7"

# Which subpath of the big compound path is which glyph. Indices are into the
# flattened subpath list built below; s0 is the sticker silhouette and every
# other entry is a letter-shaped hole in it. Verified against the rendered
# artwork -- note the source y axis is flipped, so an i-dot has a HIGHER y than
# its stem. The two counters arrive as separate top-level paths and become
# holes in their own 'e' rather than in the sticker.
GLYPHS = [
    ("mister", "M",  ["p0.s3"]),
    ("mister", "i",  ["p0.s5", "p0.s1"]),          # stem, dot
    ("mister", "S",  ["p0.s2"]),
    ("mister", "T",  ["p0.s4"]),
    ("mister", "e",  ["p0.s6", "p1.s0"]),          # outline, counter
    ("mister", "r",  ["p0.s7"]),
    ("zine",   "Z",  ["p0.s9"]),
    ("zine",   "i",  ["p0.s12", "p0.s8"]),         # stem, dot
    ("zine",   "n",  ["p0.s11"]),
    ("zine",   "e",  ["p0.s10", "p2.s0"]),         # outline, counter
]
SILHOUETTE = "p0.s0"


def load_subpaths():
    """Flatten the source into {'pN.sM': Path}, with the group transform baked
    in and the ink moved to the origin. Returns the subpaths and the ink size."""
    paths = [parse_path(n.attrib["d"])
             for n in ET.parse(SRC).iter(SVG_NS + "path")]

    subs = {}
    for pi, p in enumerate(paths):
        for si, sub in enumerate(p.continuous_subpaths()):
            # translate(0,600) scale(0.1,-0.1): the 600 is undone by the crop.
            subs[f"p{pi}.s{si}"] = sub.scaled(0.1, -0.1)

    xs = [v for s in subs.values() for v in s.bbox()[:2]]
    ys = [v for s in subs.values() for v in s.bbox()[2:]]
    x0, y0, x1, y1 = min(xs), min(ys), max(xs), max(ys)

    subs = {k: v.translated(complex(-x0, -y0)) for k, v in subs.items()}
    return subs, (x1 - x0, y1 - y0)


def d_of(*subpaths):
    """Serialize subpaths as one compound path, rounded to 3dp.

    Each subpath is emitted in full and then explicitly closed. Do NOT reach
    for svgpathtools' use_closed_attrib here: it drops the final segment and
    leans on 'Z' to close the loop, but 'Z' draws a straight line, and all but
    one of these potrace subpaths end on a curve. That silently swaps the
    closing curve for its chord -- a crescent-shaped bite out of the glyph,
    worst on the Z (20 units off) and plainly visible on the S and the r.

    Opposite winding directions survive the round trip, so a counter stays a
    hole under the default nonzero fill rule."""
    parts = []
    for sub in subpaths:
        d = sub.d(use_closed_attrib=False)
        parts.append(f"{d} Z" if sub.isclosed() else d)
    joined = " ".join(parts)
    return re.sub(r"-?\d+\.\d+", lambda m: f"{float(m.group()):.3f}", joined)


def header(w, h, title):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" '
            f'viewBox="0 0 {w:.2f} {h:.2f}" width="{w:.2f}" height="{h:.2f}">\n'
            f"  <title>{title}</title>\n")


def write_knockout(subs, size):
    """The original mark, cleaned: one path, letters transparent."""
    w, h = size
    order = [SILHOUETTE] + [k for _, _, keys in GLYPHS for k in keys]
    body = (f'  <path id="MiSTerZine" fill="#ffffff" fill-rule="nonzero"\n'
            f'        d="{d_of(*[subs[k] for k in order])}"/>\n')
    out = OUT / "misterzine-knockout.svg"
    out.write_text(header(w, h, "MiSTerZine wordmark (knockout)") + body + "</svg>\n")
    print(f"wrote {out.relative_to(OUT.parent)}")


def write_layered(subs, size):
    """Sticker and glyphs as separate objects, each id a layer name."""
    w, h = size
    lines = [header(w, h, "MiSTerZine wordmark (layered)")]
    lines.append('  <g id="MiSTerZine">\n')
    lines.append(f'    <path id="Sticker" fill="{STICKER}"\n'
                 f'          d="{d_of(subs[SILHOUETTE])}"/>\n')

    for word in ("mister", "zine"):
        label = "MiSTer" if word == "mister" else "Zine"
        lines.append(f'    <g id="{label}" fill="{LETTERS}" fill-rule="nonzero">\n')
        for w_, glyph, keys in GLYPHS:
            if w_ != word:
                continue
            lines.append(f'      <path id="{word}-{glyph}"\n'
                         f'            d="{d_of(*[subs[k] for k in keys])}"/>\n')
        lines.append("    </g>\n")

    lines.append("  </g>\n</svg>\n")
    out = OUT / "misterzine-layered.svg"
    out.write_text("".join(lines))
    print(f"wrote {out.relative_to(OUT.parent)}")


def main():
    subs, size = load_subpaths()
    print(f"ink {size[0]:.2f} x {size[1]:.2f}  ({size[0] / size[1]:.3f}:1)")
    write_knockout(subs, size)
    write_layered(subs, size)


if __name__ == "__main__":
    main()
