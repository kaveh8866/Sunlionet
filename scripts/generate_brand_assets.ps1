$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$brandkit = Join-Path $root "Brandkit"
$srcPng = Join-Path $brandkit "sunlionet.png"
$srcIco = Join-Path $brandkit "Sunlionet.ico"

$outRoot = Join-Path $root "assets\brand"
$outWebPublic = Join-Path $root "website\public\brand"
$outWebApp = Join-Path $root "website\src\app"
$outAndroidRes = Join-Path $root "android\app\src\main\res"

if (!(Test-Path $srcPng)) { throw "Missing source png: $srcPng" }
if (!(Test-Path $srcIco)) { throw "Missing source ico: $srcIco" }

New-Item -ItemType Directory -Force -Path $outRoot | Out-Null
New-Item -ItemType Directory -Force -Path $outWebPublic | Out-Null
New-Item -ItemType Directory -Force -Path $outAndroidRes | Out-Null

Add-Type -AssemblyName System.Drawing

function New-ResizedPng {
	param(
		[Parameter(Mandatory = $true)][string]$InputPath,
		[Parameter(Mandatory = $true)][string]$OutputPath,
		[Parameter(Mandatory = $true)][int]$Size,
		[Parameter(Mandatory = $true)][System.Drawing.Imaging.ColorMatrix]$Matrix
	)

	$src = [System.Drawing.Image]::FromFile($InputPath)
	try {
		$bmp = New-Object System.Drawing.Bitmap $Size, $Size, ([System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
		try {
			$g = [System.Drawing.Graphics]::FromImage($bmp)
			try {
				$g.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
				$g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
				$g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
				$g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality

				$attrs = New-Object System.Drawing.Imaging.ImageAttributes
				$attrs.SetColorMatrix($Matrix)
				$rect = New-Object System.Drawing.Rectangle 0, 0, $Size, $Size
				$g.DrawImage($src, $rect, 0, 0, $src.Width, $src.Height, ([System.Drawing.GraphicsUnit]::Pixel), $attrs)
			} finally {
				$g.Dispose()
			}

			$dir = Split-Path -Parent $OutputPath
			New-Item -ItemType Directory -Force -Path $dir | Out-Null
			$bmp.Save($OutputPath, [System.Drawing.Imaging.ImageFormat]::Png)
		} finally {
			$bmp.Dispose()
		}
	} finally {
		$src.Dispose()
	}
}

$identity = New-Object System.Drawing.Imaging.ColorMatrix (,@(
	@(1,0,0,0,0),
	@(0,1,0,0,0),
	@(0,0,1,0,0),
	@(0,0,0,1,0),
	@(0,0,0,0,1)
))

$mono = New-Object System.Drawing.Imaging.ColorMatrix (,@(
	@(0.2126,0.2126,0.2126,0,0),
	@(0.7152,0.7152,0.7152,0,0),
	@(0.0722,0.0722,0.0722,0,0),
	@(0,0,0,1,0),
	@(0,0,0,0,1)
))

$invert = New-Object System.Drawing.Imaging.ColorMatrix (,@(
	@(-1,0,0,0,0),
	@(0,-1,0,0,0),
	@(0,0,-1,0,0),
	@(0,0,0,1,0),
	@(1,1,1,0,1)
))

$sizes = @(16, 32, 48, 64, 128, 256, 512)

Copy-Item -Force $srcIco (Join-Path $outRoot "sunlionet.ico")
Copy-Item -Force $srcIco (Join-Path $outWebApp "favicon.ico")
Copy-Item -Force $srcIco (Join-Path $outWebApp "favcon.ico")
Copy-Item -Force $srcIco (Join-Path $root "website\public\favicon.ico")

foreach ($s in $sizes) {
	$colorOut = Join-Path $outRoot ("sunlionet-color-{0}.png" -f $s)
	$monoOut = Join-Path $outRoot ("sunlionet-mono-{0}.png" -f $s)
	$invOut = Join-Path $outRoot ("sunlionet-invert-{0}.png" -f $s)

	New-ResizedPng -InputPath $srcPng -OutputPath $colorOut -Size $s -Matrix $identity
	New-ResizedPng -InputPath $srcPng -OutputPath $monoOut -Size $s -Matrix $mono
	New-ResizedPng -InputPath $srcPng -OutputPath $invOut -Size $s -Matrix $invert

	Copy-Item -Force $colorOut (Join-Path $outWebPublic ("sunlionet-color-{0}.png" -f $s))
	Copy-Item -Force $monoOut (Join-Path $outWebPublic ("sunlionet-mono-{0}.png" -f $s))
	Copy-Item -Force $invOut (Join-Path $outWebPublic ("sunlionet-invert-{0}.png" -f $s))
}

Copy-Item -Force (Join-Path $outRoot "sunlionet-color-512.png") (Join-Path $outWebApp "icon.png")

$androidMipmap = @{
	"mipmap-mdpi"    = 48
	"mipmap-hdpi"    = 72
	"mipmap-xhdpi"   = 96
	"mipmap-xxhdpi"  = 144
	"mipmap-xxxhdpi" = 192
}

foreach ($kv in $androidMipmap.GetEnumerator()) {
	$dir = Join-Path $outAndroidRes $kv.Key
	$size = [int]$kv.Value
	New-Item -ItemType Directory -Force -Path $dir | Out-Null
	New-ResizedPng -InputPath $srcPng -OutputPath (Join-Path $dir "ic_launcher.png") -Size $size -Matrix $identity
	New-ResizedPng -InputPath $srcPng -OutputPath (Join-Path $dir "ic_launcher_round.png") -Size $size -Matrix $identity
}
