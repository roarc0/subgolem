#!/bin/bash
set -e

# Define the local model we want to use inside subgolem for refinement
# qwen2.5:7b is chosen here because it is exceptionally strong at strict formatting
# tasks and JSON/SRT generation without adding conversational filler.
DEFAULT_MODEL="qwen2.5:7b"

echo "========================================="
echo "🦙 Setting up Local Ollama + $DEFAULT_MODEL"
echo "========================================="

# 1. Check if Ollama is installed
if ! command -v ollama &> /dev/null; then
    echo "Ollama is not installed. Installing now..."
    curl -fsSL https://ollama.com/install.sh | sh
else
    echo "✓ Ollama is already installed."
fi

# 2. Check if the Ollama service is running
echo "Checking if Ollama service is reachable..."
if ! curl -s http://localhost:11434/api/tags &> /dev/null; then
    echo "Starting Ollama service..."
    # Depending on the system, systemctl may be active. Otherwise run in background.
    if command -v systemctl &> /dev/null; then
        sudo systemctl start ollama || {
            echo "Failed to start via systemctl, trying direct background execution..."
            nohup ollama serve > /tmp/ollama.log 2>&1 &
            sleep 3
        }
    else
        nohup ollama serve > /tmp/ollama.log 2>&1 &
        sleep 3
    fi
else
    echo "✓ Ollama service is running."
fi

# 3. Pull the required model
echo "Pulling the local LLM model '$DEFAULT_MODEL' (this might take a while)..."
ollama pull "$DEFAULT_MODEL"

# 4. Check if we need to align config.yaml
CONFIG_FILE="config.yaml"
if [ -f "$CONFIG_FILE" ]; then
    echo "Aligning config.yaml to use $DEFAULT_MODEL..."
    # We use sed to replace the model specifically inside the llm_refine block
    # Ensure standard sed pattern match for the base ollama setup
    sed -i "s/model: *\"[^\"]*\"/model: \"$DEFAULT_MODEL\"/g" "$CONFIG_FILE"
    echo "✓ config.yaml aligned."
else
    echo "config.yaml not found in current directory. Run this from the root of subgolem."
fi

echo "========================================="
echo "🎉 Setup complete!"
echo "Your offline LLM refiner is ready."
echo "You can now run: ./bin/subgolem -i data/test.mp4"
echo "========================================="
