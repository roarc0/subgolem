#!/bin/bash
set -e

LLAMACPP_DIR="data/llama-src/llama.cpp"
MODEL_DIR="data/models"
MODEL_FILE="qwen2.5-7b-instruct-q4_k_m.gguf"
MODEL_URL="https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/${MODEL_FILE}"
SERVER_PORT=8080

echo "================================================="
echo "🦙 Setting up llama.cpp (Vulkan GPU) + Qwen2.5-7B"
echo "================================================="

# 1. Clone or update llama.cpp
if [ ! -d "$LLAMACPP_DIR" ]; then
    echo "Cloning llama.cpp..."
    mkdir -p "$(dirname "$LLAMACPP_DIR")"
    git clone --depth 1 https://github.com/ggerganov/llama.cpp "$LLAMACPP_DIR"
else
    echo "✓ llama.cpp source already present."
fi

# 2. Build with Vulkan (same backend whisper.cpp uses)
LLAMA_BIN="$LLAMACPP_DIR/build/bin/llama-server"
if [ ! -f "$LLAMA_BIN" ]; then
    echo "Building llama.cpp with Vulkan support..."
    cmake -S "$LLAMACPP_DIR" -B "$LLAMACPP_DIR/build" \
        -DCMAKE_BUILD_TYPE=Release \
        -DGGML_VULKAN=ON \
        -DLLAMA_BUILD_TESTS=OFF \
        -DLLAMA_BUILD_EXAMPLES=OFF \
        -DLLAMA_SERVER=ON
    cmake --build "$LLAMACPP_DIR/build" -j"$(nproc)" --target llama-server
    echo "✓ llama-server built."
else
    echo "✓ llama-server already built."
fi

# 3. Download the model if missing
mkdir -p "$MODEL_DIR"
if [ ! -f "$MODEL_DIR/$MODEL_FILE" ]; then
    echo "Downloading $MODEL_FILE (~4.7 GB)..."
    curl -L --progress-bar "$MODEL_URL" -o "$MODEL_DIR/$MODEL_FILE"
    echo "✓ Model downloaded."
else
    echo "✓ Model already present at $MODEL_DIR/$MODEL_FILE"
fi

# 4. Update config.yaml to use llamacpp backend
CONFIG_FILE="config.yaml"
if [ -f "$CONFIG_FILE" ]; then
    echo "Switching config.yaml to llamacpp backend..."
    sed -i 's/backend: *"ollama"/backend: "llamacpp"/' "$CONFIG_FILE"
    echo "✓ config.yaml updated."
fi

echo ""
echo "================================================="
echo "🎉 Setup complete!"
echo ""
echo "Start the LLM server with:"
echo "  $LLAMA_BIN \\"
echo "    -m $MODEL_DIR/$MODEL_FILE \\"
echo "    -ngl 99 \\"
echo "    --port $SERVER_PORT \\"
echo "    --ctx-size 4096"
echo ""
echo "Then run subgolem as usual:"
echo "  ./bin/subgolem -i data/test.mp4"
echo ""
echo "To switch back to Ollama:"
echo "  Set  backend: \"ollama\"  in config.yaml"
echo "================================================="
