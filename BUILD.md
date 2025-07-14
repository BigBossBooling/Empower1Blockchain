# Building EmPower1 Blockchain

This document describes how to build the EmPower1 Blockchain from source.

## Prerequisites

* Go version 1.20 or later

## Building

1. **Clone the repository:**

   ```bash
   git clone https://github.com/your-username/empower1.git
   ```

2. **Navigate to the project directory:**

   ```bash
   cd empower1
   ```

3. **Build the project:**

   ```bash
   go build ./...
   ```

4. **Run the application:**

   ```bash
   ./empower1d
   ```

## Continuous Integration

This project uses GitHub Actions for continuous integration. The build process is defined in the `.github/workflows/build.yml` file. The workflow is triggered on every push and pull request to the `main` branch. It builds and tests the project to ensure that it is always in a working state.
