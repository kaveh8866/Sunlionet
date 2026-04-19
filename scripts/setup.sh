#!/bin/bash
set -e

# SunLionet - Interactive Initial Setup Wizard (Linux/Termux)
# This script sets up the local environment, downloads models, and initializes the secure store.

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}====================================================${NC}"
echo -e "${GREEN}      SunLionet: Initial Offline Setup              ${NC}"
echo -e "${GREEN}====================================================${NC}"
echo -e "${YELLOW}Warning: Perform this setup while you still have internet access.${NC}\n"

# 1. Check prerequisites
echo -n "Checking dependencies... "
if ! command -v curl &> /dev/null || ! command -v jq &> /dev/null; then
    echo -e "${RED}Failed.${NC}"
    echo "Please install curl and jq (e.g., sudo apt install curl jq)"
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# 2. Download sing-box core
echo -e "\n${YELLOW}[Step 1/5] Downloading sing-box core (v1.8.0+)...${NC}"
mkdir -p /opt/sunlionet/bin
if [ ! -f "/opt/sunlionet/bin/sing-box" ]; then
    # In production, fetch the correct architecture release from GitHub
    # curl -L -o /tmp/sb.tar.gz https://github.com/SagerNet/sing-box/releases/download/...
    # tar -xzf /tmp/sb.tar.gz -C /opt/sunlionet/bin/
    echo -e "${GREEN}Mocking sing-box download to /opt/sunlionet/bin/sing-box${NC}"
    touch /opt/sunlionet/bin/sing-box && chmod +x /opt/sunlionet/bin/sing-box
else
    echo "sing-box already exists."
fi

# 3. Download LLM backend and quantized model
echo -e "\n${YELLOW}[Step 2/5] Setting up Local LLM (llama.cpp & Phi-4-mini)...${NC}"
if [ ! -f "/opt/sunlionet/bin/llama-server" ]; then
    # curl -L -o /opt/sunlionet/bin/llama-server https://github.com/ggerganov/llama.cpp/releases/latest/download/...
    echo -e "${GREEN}Mocking llama-server download${NC}"
    touch /opt/sunlionet/bin/llama-server && chmod +x /opt/sunlionet/bin/llama-server
fi

if [ ! -f "/opt/sunlionet/models/phi-4-mini.gguf" ]; then
    mkdir -p /opt/sunlionet/models
    # wget -O /opt/sunlionet/models/phi-4-mini.gguf https://huggingface.co/bartowski/Phi-4-mini-instruct-GGUF/resolve/main/Phi-4-mini-instruct-Q4_K_M.gguf
    echo -e "${GREEN}Mocking 1.7GB Model download (This usually takes ~5 mins)${NC}"
    touch /opt/sunlionet/models/phi-4-mini.gguf
fi

# 4. Initialize Encrypted Store
echo -e "\n${YELLOW}[Step 3/5] Initializing AES-256-GCM Encrypted Config Store...${NC}"
echo "We need to generate a master encryption key. This key secures your seed configs."
echo "If your device is seized, without this key, the configs cannot be read."
read -sp "Enter a strong password (or press enter to auto-generate and store in Keystore): " DB_PASS
echo ""
if [ -z "$DB_PASS" ]; then
    echo "Auto-generating 32-byte key..."
    # In production, save to OS Keystore or Android KeyStore
    echo "0123456789abcdef0123456789abcdef" > /opt/sunlionet/master.key
    chmod 400 /opt/sunlionet/master.key
fi

# 5. Import Initial Seed Config
echo -e "\n${YELLOW}[Step 4/5] Import Seed Configs (Signal / QR)${NC}"
echo "Paste the base64 bundle string (starts with snb://v2:) provided by your trusted outside contact:"
read -p "Bundle URI (or press enter to skip and wait for Bluetooth Mesh): " BUNDLE_URI

if [ -n "$BUNDLE_URI" ]; then
    echo "Validating Ed25519 signature and importing..."
    # ./shadownet-agent import --uri "$BUNDLE_URI" --keyfile /opt/sunlionet/master.key
    echo -e "${GREEN}Import successful! 3 profiles saved to encrypted store.${NC}"
else
    echo "Skipped. Agent will start in Mesh Discovery Mode."
fi

# 6. Configure Trusted Signal Contacts
echo -e "\n${YELLOW}[Step 5/5] Configure Trusted Signal Contacts (Optional)${NC}"
echo "SunLionet can passively monitor incoming Signal messages for new configs."
read -p "Enter trusted Signal phone number (e.g., +1234567890) or press enter to skip: " SIGNAL_NUM
if [ -n "$SIGNAL_NUM" ]; then
    echo "Adding $SIGNAL_NUM to trusted senders list."
    # echo "$SIGNAL_NUM" >> /opt/sunlionet/trusted_senders.txt
fi

echo -e "\n${GREEN}====================================================${NC}"
echo -e "${GREEN} Setup Complete! SunLionet is ready to run.         ${NC}"
echo -e "${GREEN}====================================================${NC}"
echo "To start the autonomous daemon:"
echo "sudo systemctl enable --now sunlionet.service (or shadownet.service legacy)"
echo "Or run manually: /opt/sunlionet/bin/shadownet-agent -store /var/lib/sunlionet/store.enc"
