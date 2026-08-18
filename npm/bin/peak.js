#!/usr/bin/env node

const os = require("os");
const path = require("path");
const { spawn } = require("child_process");

const platform = os.platform();
const arch = os.arch();

const binaries = {
    "linux-x64": "peak-linux-amd64",
    "linux-arm64": "peak-linux-arm64",
    "darwin-x64": "peak-darwin-amd64",
    "darwin-arm64": "peak-darwin-arm64",
    "win32-x64": "peak-windows-amd64.exe"
};

const key = `${platform}-${arch}`;
const binary = binaries[key];

if (!binary) {
    console.error(`Unsupported platform: ${platform} ${arch}`);
    process.exit(1);
}

const binaryPath = path.join(__dirname, "..", "dist", binary);

const child = spawn(binaryPath, process.argv.slice(2), {
    stdio: "inherit"
});

child.on("error", (error) => {
    console.error(`Failed to start Peak: ${error.message}`);
    process.exit(1);
});

child.on("exit", (code) => {
    process.exit(code ?? 1);
});
