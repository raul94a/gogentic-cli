# Gogentic-CLI

Gogentic-CLI is a fully private, terminal-based user interface (TUI) designed to interact seamlessly with local Large Language Models. Built completely in Go, this tool ensures absolute data sovereignty by running inference locally without relying on third-party APIs or telemetry. 

## Core Features

*   **Absolute Privacy:** Connects directly to your local AI engine, guaranteeing that your prompts, creative writing, and injected files never leave your machine.
*   **Dynamic File Context Injection:** Features an interactive `@` autocompletion system. Type `@` followed by a file or directory name, use the `Tab` key to navigate your local filesystem, and instantly inject the exact contents of those files directly into the LLM's context window.
*   **Infinite Scroll & Robust TUI:** Built on the Elm architecture using Charmbracelet's `bubbletea`, `lipgloss`, and `viewport` components. The interface guarantees tear-free, anti-duplication resizing with full mouse and keyboard scroll support for long chat histories.
*   **Asynchronous Processing:** Includes a background queueing system allowing you to stack multiple prompts while the model is currently streaming a response.
*   **Massive Context Support:** Optimized for models handling up to 128,000 tokens, maintaining stability without silently dropping user prompts.

## Repository Structure

The project is modularized into distinct packages to separate state management, API requests, and utility functions[cite: 1]:

*   `main.go`: The primary entry point initializing the Bubble Tea application[cite: 1].
*   `tui/`: Contains the core interface logic, split into `tui_init.go`, `tui_model.go`, `tui_update.go`, and `tui_view.go` for precise lifecycle management[cite: 1].
*   `ollama/`: Manages HTTP streaming and payload formatting for the local Ollama instance, located in `models.go`[cite: 1].
*   `config/`: Handles application environment and constant variables via `config.go`[cite: 1].
*   `utils/`: Contains helper functions such as `search_files.go` for the file injection dropdown system[cite: 1].
*   `.env`, `.gitignore`, `go.mod`, and `go.sum`: Standard Go dependency management and environment configuration files[cite: 1].

## Prerequisites

*   **Go:** Version 1.21 or higher.
*   **Ollama:** Installed and running locally on port `11434`.
*   **Model:** The application defaults to `gemma4:e4b`. Ensure it is pulled to your local machine via `ollama run gemma4:e4b` before launching.

## Installation and Setup

1. Clone the repository to your local machine.
2. Download the required Charmbracelet dependencies:
   ```bash
   go mod tidy