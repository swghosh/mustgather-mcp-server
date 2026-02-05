# Must-Gather MCP Server Setup Guide

This guide provides step-by-step instructions to set up and use the Must-Gather MCP Server with an AI agent to analyze OpenShift must-gather bundles.

## Prerequisites

For collecting must-gather,
- Access to an OpenShift cluster (for collecting must-gather)
- `oc` CLI installed and configured

For running analysis on must-gather:
- An LLM agent that supports MCP server tools
- `omc` present in `$PATH`

---

## Step 1: Install omc CLI

The `omc` (OpenShift Must-Gather Client) CLI is required for the MCP server to analyze must-gather bundles.


```bash
# Navigate to a directory in your $PATH (e.g., /usr/local/bin or ~/bin)
cd /usr/local/bin

# Download and install the latest omc release
curl -sL https://github.com/gmeghnag/omc/releases/latest/download/omc_$(uname)_$(uname -m).tar.gz | tar xzf - omc && chmod +x ./omc

# Verify installation
omc --help
```


---

## Step 2: Install an MCP-Compatible Agent

Choose one of the following AI agents that support MCP (Model Context Protocol):

### Option A: Claude Code (Recommended)

Install Claude Code CLI:

```bash
npm install -g @anthropic-ai/claude-code
```

Or follow the installation instructions at: https://docs.anthropic.com/en/docs/claude-code

### Option B: Gemini CLI

Install Gemini CLI:

```bash
npm install -g @anthropic-ai/gemini-cli
```

Or follow the installation instructions at: https://github.com/google-gemini/gemini-cli

### Option C: Goose

Install Goose by following the instructions at: https://github.com/block/goose

---

## Step 3: Collect Must-Gather from Your Cluster

Ensure you are logged into your OpenShift cluster and run:

```bash
oc adm must-gather 

oc adm must-gather --dest-dir=./must-gather-output
```

---

## Step 4: Download the must-gather MCP Server

Download the must-gather MCP server:

```bash
wget -O ~/mustgather-mcp-server https://storage.googleapis.com/swghosh-01/mustgather-mcp-server-_$(uname)_$(uname -m)
```

Note the full path to your `mustgather-mcp-server` binary, e.g., `/home/user/mustgather-mcp-server`

---

## Step 5: Configure Your Agent with the MCP Server

### For Claude Code

Create or edit `~/.claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "mustgather": {
      "command": "/home/user/mustgather-mcp-server"
    }
  }
}
```

Alternatively, configure via Claude Code settings:
```bash
claude mcp add mustgather ~/mustgather-mcp-server
```

### For Gemini CLI

Create or edit `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "mustgather": {
      "command": "/home/user/mustgather-mcp-server",
      "cwd": "./",
      "timeout": 900,
      "trust": true
    }
  }
}
```

### For Goose

First, configure `~/.config/goose/config.yaml`:

```yaml
extensions:
  mustgather:
    description: 'Must-Gather MCP Server'
    enabled: true
    env_keys: []
    envs: {}
    name: mustgather
    timeout: 300
    type: stdio
```

---

## Step 6: Start the Agent and Analyze Must-Gather

Select a must-gather to analyse.

```bash
omc use /path/to/collected-must-gather

omc use https://some-link.io/where/must-gather-is-present/gathered.tar.gz
```

### With Claude Code

```bash
claude
```

Once in the Claude Code session:

```

> from the must-gather, Get all pods in the openshift-etcd namespace 

> Check etcd cluster health

> Show me any pods that are not running or have restarts

> What events are related to errors in the cluster?
```

### With Gemini CLI

```bash
gemini
```

### With Goose

```bash
goose session start
```

---

## Example Troubleshooting Session

Here's an example workflow for analyzing a cluster issue:

```
   > use must-gather from "/path/to/must-gather-collected"

   > Get all clusteroperators and show me any that are degraded

   > Get all pods in all namespaces that are not Running

   > Get all events with type Warning

   > Check etcd health and status

   > Get logs for pod api-server-xyz in namespace openshift-apiserver
```

---

## Troubleshooting

### omc command not found

Ensure `omc` is in your PATH:
```bash
which omc
# If not found, add it to PATH or reinstall
export PATH=$PATH:$(go env GOPATH)/bin
```

### MCP server not connecting

1. Verify the binary path in your config is correct and absolute
2. Check the binary has execute permissions: `chmod +x mustgather-mcp-server`
3. Try running the server manually to check for errors: `./mustgather-mcp-server`

### Must-gather not loading

Ensure the must-gather directory structure is correct:
```bash
ls /path/to/must-gather-output/
# Should contain directories like: quay-io-openshift-release-dev-xxx/ inside
```

---

## Demos

- [Demo 1: Basic Usage](https://asciinema.org/a/mvb4GGUfaAuUhMggBLrsmBBhe)
- [Demo 2: Advanced Analysis](https://asciinema.org/a/xqbgtCi3QIqfAmioeTZNurJ0P)
