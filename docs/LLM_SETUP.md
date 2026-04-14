# ShadowNet Agent: Local LLM Setup Guide

The ShadowNet Agent relies on a completely offline, quantized Large Language Model to make intelligent protocol rotation decisions against DPI. 
We strongly recommend **Phi-4-mini (3.8B)** or **Gemma-3-4B** due to their excellent reasoning capabilities at very small parameter counts.

## 1. Quantization Recommendation

To meet the hard constraint of **< 800MB peak RAM usage** (LLM + `sing-box` + detection suite), you must use heavily quantized GGUF models.

**Recommended Level:** `Q3_K_S` or `Q4_K_M`
- **Why?** A 3.8B model at `Q3_K_S` (3-bit quantization) takes roughly **1.7 GB** of disk space, but when loaded with a minimal context window (2K tokens), the peak RAM overhead can be squeezed down to ~2GB.
- *Note:* If 800MB total system RAM is a hard physical limit (e.g., an extremely cheap Android phone), you must step down to a **1.5B model** (like Qwen2.5-1.5B at `Q4_K_M`, which requires ~1.1GB RAM) or run the Go-based deterministic fallback engine without the LLM active.

## 2. Setup on Linux (Ollama / llama.cpp)

For development and standard Linux deployments, `llama.cpp`'s server mode is the most lightweight option.

### Installation
```bash
# 1. Download pre-compiled llama-server binary
curl -L -o llama-server https://github.com/ggerganov/llama.cpp/releases/latest/download/llama-server-b4146-bin-linux-x64
chmod +x llama-server

# 2. Download the Quantized Model
wget https://huggingface.co/bartowski/Phi-4-mini-instruct-GGUF/resolve/main/Phi-4-mini-instruct-Q4_K_M.gguf
```

### Runtime Command (Optimized for RAM & Speed)
```bash
./llama-server \
  -m Phi-4-mini-instruct-Q4_K_M.gguf \
  -c 2048 \           # Restrict context window to 2K tokens (saves massive RAM)
  --threads 4 \       # Limit CPU usage
  --port 8080 \       # Localhost only API
  --mlock \           # Lock model in RAM (prevents slow swapping)
  -cb                 # Continous batching for faster response
```
The ShadowNet Go agent (`cmd/inside/daemon.go`) connects to `http://127.0.0.1:8080/completion` automatically.

## 3. Setup on Android (Termux)

Android requires compiling `llama.cpp` natively or using `MLC LLM`. For a unified Go + LLM architecture, Termux + `llama.cpp` is the most robust rootless path.

### Installation via Termux
```bash
pkg update && pkg upgrade
pkg install clang wget git make cmake

# Clone and build
git clone https://github.com/ggerganov/llama.cpp
cd llama.cpp
make -j4

# Download model to Termux storage
wget https://huggingface.co/bartowski/Phi-4-mini-instruct-GGUF/resolve/main/Phi-4-mini-instruct-Q4_K_M.gguf
```

### Runtime Command (Android Battery Optimized)
```bash
./llama-server -m Phi-4-mini-instruct-Q4_K_M.gguf -c 1024 --threads 2 --port 8080
```
*Note: The Go daemon automatically detects battery levels. If `< 20%`, it will stop querying the LLM and rely strictly on deterministic failovers to save power.*

## 4. Error Handling and Fallbacks

1. **LLM Crashes or OOM Kills**: If Android's Low Memory Killer (LMK) kills `llama-server`, the Go HTTP client times out after 10s. The agent's `RotationManager` catches this and instantly defaults to a deterministic `Round-Robin` rotation of the available healthy profiles.
2. **Hallucination Prevention**: The Go client enforces a strict **GBNF Grammar** during the API call. The LLM *physically cannot* output anything other than valid JSON matching the `LLMDecision` struct.
3. **Watchdog**: The Go daemon runs the rotation loop in a separate goroutine. If the main loop panics, `systemd` (Linux) or the foreground Android Service restarts it instantly.
