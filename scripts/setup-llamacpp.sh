#!/bin/bash
set -e

MODEL_DIR="data/models"
BIN_DIR="data/llama-bin"
MODEL_FILE="qwen2.5-7b-instruct-q4_k_m.gguf"
MODEL_URL="https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/${MODEL_FILE}"
SERVER_PORT=8080

echo "================================================="
echo "🦙 Setting up llama.cpp (prebuilt Vulkan) + Qwen2.5-7B"
echo "================================================="

# 1. Fetch the latest llama.cpp release tag from GitHub
echo "Looking up latest llama.cpp release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/ggerganov/llama.cpp/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/')
echo "Latest release: $LATEST"

# 2. Download the prebuilt Linux + Vulkan binary bundle
mkdir -p "$BIN_DIR"
ARCHIVE="llama-${LATEST}-bin-ubuntu-vulkan-x64.zip"
ARCHIVE_URL="https://github.com/ggerganov/llama.cpp/releases/download/${LATEST}/${ARCHIVE}"

if [ ! -f "$BIN_DIR/llama-server" ]; then
    echo "Downloading prebuilt binary: $ARCHIVE ..."
    curl -L --progress-bar "$ARCHIVE_URL" -o "$BIN_DIR/$ARCHIVE"
    unzip -jo "$BIN_DIR/$ARCHIVE" "*/llama-server" -d "$BIN_DIR/"
    chmod +x "$BIN_DIR/llama-server"
    rm "$BIN_DIR/$ARCHIVE"
    echo "✓ llama-server ready at $BIN_DIR/llama-server"
else
    echo "✓ llama-server already present."
fi

# 3. Download the GGUF model if missing
mkdir -p "$MODEL_DIR"
if [ ! -f "$MODEL_DIR/$MODEL_FILE" ]; then
    echo "Downloading $MODEL_FILE (~4.7 GB)..."
    curl -L --progress-bar "$MODEL_URL" -o "$MODEL_DIR/$MODEL_FILE"
    echo "✓ Model downloaded."
else
    echo "✓ Model already present at $MODEL_DIR/$MODEL_FILE"
fi

# 4. Switch config.yaml to llamacpp backend
CONFIG_FILE="config.yaml"
if [ -f "$CONFIG_FILE" ]; then
    sed -i 's/backend: *"ollama"/backend: "llamacpp"/' "$CONFIG_FILE"
    echo "✓ config.yaml: backend set to llamacpp"
fi

echo ""
echo "================================================="
echo "🎉 Setup complete! Start the server with:"
echo ""
echo "  $BIN_DIR/llama-server \\"
echo "    -m $MODEL_DIR/$MODEL_FILE \\"
echo "    -ngl 99 --port $SERVER_PORT --ctx-size 4096"
echo ""
echo "Then run: ./bin/subgolem -i <your_video.mp4>"
echo "================================================="
