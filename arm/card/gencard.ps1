# Test card for judging fonts and poster sizes on the CRT (composite vs Y/C).
# Writes card_a.ppm (text drawn as-is) and card_b.ppm (text stretched 9/8
# horizontally to cancel the 8/9 pixel aspect of 720x480). Posters come from
# p<i>_<w>.ppm in this folder (fetched from the server by the driver script).
#   powershell -File arm\card\gencard.ps1
Add-Type -AssemblyName System.Drawing
$here = $PSScriptRoot
$W = 720; $H = 480
$BG = [System.Drawing.Color]::FromArgb(0x10, 0x18, 0x28)

function ReadPPM($path) {
  $b = [System.IO.File]::ReadAllBytes($path)
  # P6\n<w> <h>\n255\n
  $i = 0; $tok = @()
  while ($tok.Count -lt 4) {
    while ([char]$b[$i] -match '\s') { $i++ }
    $s = $i; while (-not ([char]$b[$i] -match '\s')) { $i++ }
    $tok += [System.Text.Encoding]::ASCII.GetString($b, $s, $i - $s)
  }
  $i++
  $w = [int]$tok[1]; $h = [int]$tok[2]
  $bmp = New-Object System.Drawing.Bitmap($w, $h, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
  $rect = New-Object System.Drawing.Rectangle(0, 0, $w, $h)
  $d = $bmp.LockBits($rect, [System.Drawing.Imaging.ImageLockMode]::WriteOnly, $bmp.PixelFormat)
  $row = New-Object byte[] ($w * 3)
  for ($y = 0; $y -lt $h; $y++) {
    for ($x = 0; $x -lt $w; $x++) { $o = $i + ($y * $w + $x) * 3; $row[$x*3] = $b[$o+2]; $row[$x*3+1] = $b[$o+1]; $row[$x*3+2] = $b[$o] }
    [System.Runtime.InteropServices.Marshal]::Copy($row, 0, [IntPtr]::Add($d.Scan0, $y * $d.Stride), $w * 3)
  }
  $bmp.UnlockBits($d)
  return $bmp
}

function WritePPM($bmp, $path) {
  $w = $bmp.Width; $h = $bmp.Height
  $rect = New-Object System.Drawing.Rectangle(0, 0, $w, $h)
  $d = $bmp.LockBits($rect, [System.Drawing.Imaging.ImageLockMode]::ReadOnly, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
  $hdr = [System.Text.Encoding]::ASCII.GetBytes("P6`n$w $h`n255`n")
  $out = New-Object byte[] ($hdr.Length + $w * $h * 3)
  [Array]::Copy($hdr, $out, $hdr.Length)
  $row = New-Object byte[] ($d.Stride)
  $o = $hdr.Length
  for ($y = 0; $y -lt $h; $y++) {
    [System.Runtime.InteropServices.Marshal]::Copy([IntPtr]::Add($d.Scan0, $y * $d.Stride), $row, 0, $d.Stride)
    for ($x = 0; $x -lt $w; $x++) { $out[$o] = $row[$x*3+2]; $out[$o+1] = $row[$x*3+1]; $out[$o+2] = $row[$x*3]; $o += 3 }
  }
  $bmp.UnlockBits($d)
  [System.IO.File]::WriteAllBytes($path, $out)
}

function Card($stretch, $path) {
  $bmp = New-Object System.Drawing.Bitmap($W, $H, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.Clear($BG)
  $g.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit
  $fmt = New-Object System.Drawing.StringFormat([System.Drawing.StringFormat]::GenericTypographic)
  $white = [System.Drawing.Brushes]::White
  $sx = 1.0; if ($stretch) { $sx = 1.125 }
  function T($txt, $x, $y, $fam, $px, $style, $brush) {
    $f = New-Object System.Drawing.Font($fam, [float]$px, $style, [System.Drawing.GraphicsUnit]::Pixel)
    $g.ResetTransform(); $g.ScaleTransform([float]$sx, 1.0)
    $g.DrawString($txt, $f, $brush, [float]($x / $sx), [float]$y, $fmt)
    $g.ResetTransform()
  }
  $R = [System.Drawing.FontStyle]::Regular; $B = [System.Drawing.FontStyle]::Bold
  $faces = @(
    @('Roboto', $R, 'Regular'), @('Roboto Medium', $R, 'Medium'), @('Roboto', $B, 'Bold'),
    @('Roboto Condensed', $R, 'Cond Reg'), @('Roboto Condensed', $B, 'Cond Bold'))
  $sizes = @(14, 16, 18, 20, 22, 26)
  $label = 'A'; if ($stretch) { $label = 'B  (text stretched 9/8)' }
  T "Card $label   columns: Regular / Medium / Bold / Cond Regular / Cond Bold   rows: 14 16 18 20 22 26 px" 36 22 'Roboto' 13 $R ([System.Drawing.Brushes]::LightGray)
  $colx = @(36, 176, 316, 456, 580)
  $y = 40
  foreach ($px in $sizes) {
    for ($c = 0; $c -lt 5; $c++) {
      T "Rogue $px" $colx[$c] $y $faces[$c][0] $px $faces[$c][1] $white
    }
    $y += [int]($px * 1.15) + 2
    if ($y % 2) { $y++ }
  }
  # colour row: same 18 px Medium in greys and colours
  $y += 4
  $cols = @(
    @('white', [System.Drawing.Color]::White), @('C0 grey', [System.Drawing.Color]::FromArgb(0xC0,0xC0,0xC0)),
    @('80 grey', [System.Drawing.Color]::FromArgb(0x80,0x80,0x80)), @('amber', [System.Drawing.Color]::FromArgb(0xE5,0xA0,0x0D)),
    @('dull amber', [System.Drawing.Color]::FromArgb(0xC8,0xA0,0x50)), @('red', [System.Drawing.Color]::FromArgb(0xF0,0x30,0x30)),
    @('cyan', [System.Drawing.Color]::FromArgb(0x30,0xC0,0xF0)))
  $x = 36
  foreach ($cc in $cols) { T $cc[0] $x $y 'Roboto Medium' 18 $R (New-Object System.Drawing.SolidBrush($cc[1])); $x += 96 }
  $y += 28
  # posters: 90, 110, 135, 160 wide, cropped to 180 tall; left pair on black, right pair on dark grey
  $py = $y
  $g.FillRectangle((New-Object System.Drawing.SolidBrush([System.Drawing.Color]::Black)), 30, $py - 4, 260, 188)
  $g.FillRectangle((New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(0x30,0x30,0x30))), 292, $py - 4, 302, 188)
  $x = 36; $i = 0
  foreach ($w in @(90, 110, 135, 160)) {
    $p = ReadPPM (Join-Path $here "p${i}_$w.ppm")
    $ch = [Math]::Min(180, $p.Height)
    $src = New-Object System.Drawing.Rectangle(0, [int](($p.Height - $ch) / 2), $p.Width, $ch)
    $dst = New-Object System.Drawing.Rectangle($x, $py, $p.Width, $ch)
    $g.DrawImage($p, $dst, $src, [System.Drawing.GraphicsUnit]::Pixel)
    T "$w" $x ($py + 182 - 16) 'Roboto' 12 $R ([System.Drawing.Brushes]::LightGray)
    $x += $w + 12; $i++
  }
  # rules and checkerboards on the right
  $rx = 604; $ry = $py
  $pen1 = New-Object System.Drawing.Pen([System.Drawing.Color]::White, 1)
  $g.DrawLine($pen1, $rx, $ry, $rx + 80, $ry)           # 1 px line
  $g.FillRectangle($white, $rx, $ry + 8, 80, 2)          # 2 px line
  $g.FillRectangle($white, $rx, $ry + 18, 80, 4)         # 4 px line
  for ($yy = 0; $yy -lt 40; $yy++) { for ($xx = 0; $xx -lt 80; $xx++) { if ((($xx + $yy) % 2) -eq 0) { $bmp.SetPixel($rx + $xx, $ry + 30 + $yy, [System.Drawing.Color]::White) } } }
  for ($yy = 0; $yy -lt 40; $yy++) { for ($xx = 0; $xx -lt 80; $xx++) { if (((([int]($xx/2)) + ([int]($yy/2))) % 2) -eq 0) { $bmp.SetPixel($rx + $xx, $ry + 76 + $yy, [System.Drawing.Color]::White) } } }
  for ($xx = 0; $xx -lt 80; $xx += 2) { $g.FillRectangle($white, $rx + $xx, $ry + 122, 1, 20) }   # 1 px vertical stripes
  for ($xx = 0; $xx -lt 80; $xx += 4) { $g.FillRectangle($white, $rx + $xx, $ry + 148, 2, 20) }   # 2 px vertical stripes
  T "1/2/4 px  chk 1,2  v1,v2" $rx ($ry + 170) 'Roboto Condensed' 11 $R ([System.Drawing.Brushes]::LightGray)
  $y = $py + 192
  T "Continue Watching   The Bridge on the River Kwai (1957)  2h 41m   Resume at 1:12:07" 36 $y 'Roboto Medium' 17 $R $white
  $y += 22
  T "Recently Added   American Gangster (2007)  2h 37m   Lord of Illusions (1995)  1h 49m" 36 $y 'Roboto Condensed' 17 $R ([System.Drawing.Brushes]::LightGray)
  $g.Dispose()
  WritePPM $bmp $path
  "wrote $path"
}
Card $false (Join-Path $here 'card_a.ppm')
Card $true (Join-Path $here 'card_b.ppm')

# Card C: bloom test. Stretched text, faces across, sizes down, one band per brightness.
function CardC($path) {
  $bmp = New-Object System.Drawing.Bitmap($W, $H, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.Clear($BG)
  $g.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit
  $fmt = New-Object System.Drawing.StringFormat([System.Drawing.StringFormat]::GenericTypographic)
  $sx = 1.125
  function T($txt, $x, $y, $fam, $px, $style, $brush) {
    $f = New-Object System.Drawing.Font($fam, [float]$px, $style, [System.Drawing.GraphicsUnit]::Pixel)
    $g.ResetTransform(); $g.ScaleTransform([float]$sx, 1.0)
    $g.DrawString($txt, $f, $brush, [float]($x / $sx), [float]$y, $fmt)
    $g.ResetTransform()
  }
  $R = [System.Drawing.FontStyle]::Regular; $B = [System.Drawing.FontStyle]::Bold
  $faces = @(@('Roboto', $R), @('Roboto Medium', $R), @('Roboto', $B), @('Roboto Condensed', $R), @('Roboto Condensed', $B))
  $colx = @(36, 176, 316, 456, 590)
  $bands = @(@('FF', 0xFF), @('E0', 0xE0), @('D0', 0xD0), @('C0', 0xC0))
  T "Card C  (stretched)   Regular / Medium / Bold / Cond Reg / Cond Bold   rows 18 20 22 26   bands: white, E0, D0, C0 grey" 36 12 'Roboto' 13 $R ([System.Drawing.Brushes]::LightGray)
  $y = 30
  foreach ($bd in $bands) {
    $v = $bd[1]
    $br = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb($v, $v, $v))
    foreach ($px in @(18, 20, 22, 26)) {
      for ($c = 0; $c -lt 5; $c++) { T "Rogue $($bd[0])" $colx[$c] $y $faces[$c][0] $px $faces[$c][1] $br }
      $y += [int]($px * 1.1)
      if ($y % 2) { $y++ }
    }
    $y += 6
  }
  $g.Dispose()
  WritePPM $bmp $path
  "wrote $path"
}
CardC (Join-Path $here 'card_c.ppm')
