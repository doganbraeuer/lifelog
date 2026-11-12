# Starts whisper.cpp's server for the transcription (docs/API.md, 3.9) on
# Windows. The backend talks to it at WHISPER_URL
# (default http://127.0.0.1:8178); the phone never does.
#
#   powershell -File backend\scripts\whisper-server.ps1 download   # fetch and verify the models (once)
#   powershell -File backend\scripts\whisper-server.ps1            # start the server (keep it running)
#
# After the start it sends warmup.wav once, and it starts whisper-server
# again if it stops (crash), waiting 2 s, then up to 60 s if it keeps
# stopping. Ctrl-C stops both. Needs curl.exe (part of Windows 10 and 11).
#
# Settings (all optional):
#   WHISPER_MODEL_DIR  where the models are   (default %LOCALAPPDATA%\lifelog\whisper)
#   WHISPER_PORT       port                   (default 8178; 8080 is the backend)
#   WHISPER_SERVER     path to whisper-server.exe (default: found in PATH)
#
# whisper-server.exe comes with the whisper.cpp release from GitHub
# (whisper-bin-x64.zip), see backend/README.md, section speech.

param([string]$Command = "")
$ErrorActionPreference = "Stop"

$ModelDir = if ($env:WHISPER_MODEL_DIR) { $env:WHISPER_MODEL_DIR } else { Join-Path $env:LOCALAPPDATA "lifelog\whisper" }
$Port = if ($env:WHISPER_PORT) { $env:WHISPER_PORT } else { "8178" }
$Server = if ($env:WHISPER_SERVER) { $env:WHISPER_SERVER } else { "whisper-server.exe" }

# File, URL and SHA-256 as published by Hugging Face (checked 2026-11-12).
$Models = @(
  @{ File = "ggml-silero-v5.1.2.bin"; Url = "https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v5.1.2.bin"; Sha = "29940d98d42b91fbd05ce489f3ecf7c72f0a42f027e4875919a28fb4c04ea2cf" },
  @{ File = "ggml-large-v3-turbo.bin"; Url = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin"; Sha = "1fc70f774d38eb169993ac391eea357ef47c88757ef72ee5943879b7e8e2bc69" }
)

if ($Command -eq "download") {
  New-Item -ItemType Directory -Force -Path $ModelDir | Out-Null
  foreach ($m in $Models) {
    $path = Join-Path $ModelDir $m.File
    if (-not (Test-Path $path)) {
      Write-Host "Downloading $($m.File) ..."
      Invoke-WebRequest -Uri $m.Url -OutFile "$path.part"
      Move-Item "$path.part" $path
    }
    $got = (Get-FileHash -Algorithm SHA256 $path).Hash.ToLower()
    if ($got -ne $m.Sha) { throw "SHA-256 of $($m.File) does not match: $got (expected $($m.Sha)). Delete the file and download again." }
    Write-Host "$($m.File): SHA-256 ok"
  }
  exit 0
}
if ($Command -ne "") { Get-Content $PSCommandPath | Select-Object -Skip 1 -First 18; exit 2 }

foreach ($m in $Models) {
  if (-not (Test-Path (Join-Path $ModelDir $m.File))) { throw "$(Join-Path $ModelDir $m.File) is missing. Run: whisper-server.ps1 download" }
}

# German only, no temperature fallback, non-speech tokens suppressed,
# voice activity detection: only parts with speech are decoded. The backend
# sends a list of medical terms as prompt (internal/speech/whisper,
# Vocabulary); --carry-initial-prompt keeps it for every 30-second window.
# "--max-context 0" would switch the prompt off (measured 2026-11-12,
# docs/speech/README.md). Only reachable from this machine.

# Start-Process does not quote arguments: paths with spaces need quotes.
function Quote([string]$s) { if ($s -match '\s') { '"' + $s + '"' } else { $s } }
$ServerArgs = @(
  "--model", (Quote (Join-Path $ModelDir "ggml-large-v3-turbo.bin")),
  "--host", "127.0.0.1", "--port", $Port,
  "--language", "de",
  "--carry-initial-prompt",
  "--no-fallback",
  "--suppress-nst",
  "--vad", "--vad-model", (Quote (Join-Path $ModelDir "ggml-silero-v5.1.2.bin"))
)
$Url = "http://127.0.0.1:$Port"
$Warmup = Join-Path $PSScriptRoot "warmup.wav"

# Until whisper-server answers (it loads the model first); false if it
# stops or takes longer than 120 s.
function Wait-Ready($proc) {
  for ($i = 0; $i -lt 120; $i++) {
    if ($proc.HasExited) { return $false }
    try { Invoke-WebRequest -UseBasicParsing -Uri "$Url/" -TimeoutSec 2 | Out-Null; return $true } catch { Start-Sleep -Seconds 1 }
  }
  return $false
}

# One short recording with speech. whisper.cpp 1.9.5 crashes if the first
# request after the start has no speech (issue #4156, fixed after 1.9.5);
# the backend avoids it too (no_language_probabilities), this also prepares
# the GPU so the first real recording is not slower.
function Invoke-WarmUp {
  & curl.exe -fs -o NUL --max-time 60 -F "file=@$Warmup" -F language=de -F response_format=json "$Url/inference"
  return ($LASTEXITCODE -eq 0)
}

$first = $true
$delay = 2
$proc = $null
try {
  while ($true) {
    $started = Get-Date
    $proc = Start-Process -FilePath $Server -ArgumentList $ServerArgs -NoNewWindow -PassThru
    $null = $proc.Handle # without it ExitCode stays empty
    if (Wait-Ready $proc) {
      $first = $false
      if (Invoke-WarmUp) { Write-Host "whisper-server.ps1: ready at $Url (warmed up)" } else { Write-Warning "whisper-server.ps1: warm-up failed" }
    } elseif ($first) {
      throw "whisper-server did not start (see its output above)"
    }

    $proc.WaitForExit()
    $ran = [int]((Get-Date) - $started).TotalSeconds
    if ($ran -ge 300) { $delay = 2 } # it ran for a while: start again quickly
    Write-Warning "whisper-server.ps1: whisper-server stopped (exit code $($proc.ExitCode)) after $ran s; starting it again in $delay s"
    Start-Sleep -Seconds $delay
    $delay = [Math]::Min($delay * 2, 60)
  }
} finally {
  # Ctrl-C, an error or the end: do not leave whisper-server running.
  if ($proc -and -not $proc.HasExited) { $proc.Kill() }
}
