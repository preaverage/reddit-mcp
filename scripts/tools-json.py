#!/usr/bin/env python3
"""Ask a built server for its tool list and print it as manifest JSON.

Keeps the .mcpb manifest in sync with what the server actually registers.
"""

import json
import subprocess
import sys


def tool_list(binary):
    proc = subprocess.Popen(
        [binary, "serve"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
    )

    def send(message):
        proc.stdin.write(json.dumps(message) + "\n")
        proc.stdin.flush()

    send({
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2025-06-18",
            "capabilities": {},
            "clientInfo": {"name": "build", "version": "0"},
        },
    })
    send({"jsonrpc": "2.0", "method": "notifications/initialized"})
    send({"jsonrpc": "2.0", "id": 2, "method": "tools/list"})

    try:
        while True:
            line = proc.stdout.readline()
            if not line:
                raise SystemExit("server closed before listing tools")

            message = json.loads(line)
            if message.get("id") == 2:
                return message["result"]["tools"]
    finally:
        proc.terminate()


def main():
    if len(sys.argv) != 2:
        raise SystemExit("usage: tools-json.py <binary>")

    tools = [
        {"name": t["name"], "description": t.get("description", "")}
        for t in sorted(tool_list(sys.argv[1]), key=lambda t: t["name"])
    ]

    print(json.dumps(tools, indent=4))


if __name__ == "__main__":
    main()
