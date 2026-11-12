# Starts whisper.cpp's server for the transcription (docs/API.md, 3.9) on
# Windows. The backend talks to it at WHISPER_URL
# (default http://127.0.0.1:8178); the phone never does.
#
#   powershell -File backend\scripts\whisper-server.ps1 download   # fetch and verify the models (once)
#   powershell -File backend\scripts\whisper-server.ps1            # start the server (keep it running)
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
if ($Command -ne "") { Get-Content $PSCommandPath | Select-Object -Skip 1 -First 13; exit 2 }

foreach ($m in $Models) {
  if (-not (Test-Path (Join-Path $ModelDir $m.File))) { throw "$(Join-Path $ModelDir $m.File) is missing. Run: whisper-server.ps1 download" }
}

# German only, no context from earlier text (avoids loops), no temperature
# fallback, non-speech tokens suppressed, voice activity detection: only
# parts with speech are decoded. Only reachable from this machine.
& $Server `
  --model (Join-Path $ModelDir "ggml-large-v3-turbo.bin") `
  --host 127.0.0.1 --port $Port `
  --language de `
  --max-context 0 `
  --no-fallback `
  --suppress-nst `
  --vad --vad-model (Join-Path $ModelDir "ggml-silero-v5.1.2.bin")
