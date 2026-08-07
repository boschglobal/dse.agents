<!--
SPDX-FileCopyrightText: 2026 Robert Bosch GmbH

SPDX-License-Identifier: Apache-2.0
-->

# Dynamic Simulation Environment - Agents

[![CI](https://github.com/boschglobal/dse.agents/actions/workflows/ci.yaml/badge.svg)](https://github.com/boschglobal/dse.agents/actions/workflows/ci.yaml)
[![Super Linter](https://github.com/boschglobal/dse.agents/actions/workflows/super-linter.yml/badge.svg)](https://github.com/boschglobal/dse.agents/actions/workflows/super-linter.yml)
![GitHub](https://img.shields.io/github/license/boschglobal/dse.agents)


## Introduction

This repository provides an integration layer for deploying AI agents and
orchestration workflows within the Dynamic Simulation Environment (DSE) Core Platform.

This repository exposes the core capabilities of the DSE Platform, via MCP servers,
tools and models, to agentic workflows - enabling them to run, modify
and interpret the results of co-simulations which utilise the DSE Platform


### Contents

- [Usage](#usage)
  - [MCP Server](#mcp-server)
- [Development](#development)
  - [Build](#build)
  - [Packages](#packages)


### Project Structure

```text
dse.agents
├── doc/                    # Content for documentation systems
└── mcp                     # MCP Server module
    ├── api/                # API implementations for REST (Huma) and MCP
    ├── build/package/      # Dockerfiles and tool packaging
    ├── cmd/                # CLI tools
    ├── deployments/        # Deployment scripts
    ├── pkg/                # Source code
    │   └── tasks/          # Task definitions supported by the MCP server
    └── tests/              # End-to-end tests
```


## Usage

### MCP Server

#### VS Code w. Copilot CLI (WSL)

Start the MCP Server and related services:

```bash
# Optional, use local development container images.
cp mcp/deployments/.env.devel mcp/deployments/.env

# Run the docker compose deployment. See compose file for more details.
docker compose -f mcp/deployments/docker-compose.yml up -d
```

Configure the MCP Server in Copilot:

1. Start Copilot CLI Client in a terminal (`$ copilot`).
2. Add and configure the DSE MCP server with `/mcp add`
3. Provide the following configuration values when prompted:
    * Name: `dse-mcp`
    * Server Type: `HTTP`
    * Url: `http://127.0.0.1:8089`
4. Accept any remaining prompts.

The configuration file (`~/.copilot/mcp-config.json`) should look like this:

```json
{
  "mcpServers": {
    "dse-mcp": {
      "type": "http",
      "url": "http://127.0.0.1:8089",
      "tools": [
        "*"
      ]
    }
  }
}
```

#### Example Prompt

Test the operation with a prompt such as:


```copilot
> run the simulation in ./dse.agents/mcp/tests/testdata/simer/openloop/out/sim for 0.01 seconds
...
The simulation ran successfully for 0.01 seconds (21 steps at 0.0005s step size). Here's a summary:

Models: linear and input

Results (linear model output):

┌─────────┬───────┬────────┬────────┬───────────┐
│ Time    │ input │ factor │ offset │ output    │
├─────────┼───────┼────────┼────────┼───────────┤
│ 0.0000  │ 0.0   │ 0.0    │ 0.0    │ 0.0       │
├─────────┼───────┼────────┼────────┼───────────┤
│ 0.0005  │ 1.0   │ 2.0    │ 3.0    │ 5.0       │
├─────────┼───────┼────────┼────────┼───────────┤
│ 0.0010  │ -1.1  │ 2.1    │ 3.1    │ 0.79      │
├─────────┼───────┼────────┼────────┼───────────┤
│ 0.0015  │ 1.2   │ -2.2   │ 3.2    │ 0.56      │
├─────────┼───────┼────────┼────────┼───────────┤
│ 0.0020+ │ 1.3   │ 2.3    │ -3.3   │ -0.31     │
└─────────┴───────┴────────┴────────┴───────────┘

The simulation completed cleanly with no errors.
```


## Development

### Build

```bash
git clone https://github.com/boschglobal/dse.agents.git
cd dse.agents

# Build
make build
make docker

# Pull additional images used by MCP Server (optional)
docker pull pull ghcr.io/boschglobal/dse-simer:latest

# Test
make test
make test_e2e

# Additional Makefile targets
make help
```

### Packages

<!-- markdownlint-disable MD060 -->
| Package | Registry/Image | Description / Purpose |
| :--- | :--- | :--- |
| MCP | ghcr.io/boschglobal/dse-mcp | Model Context Protocol (MCP) server exposing DSE capabilities to LLMs as asynchronous tasks |
| Message | ghcr.io/boschglobal/dse-message | Echo utility to test agent task routing and integration |
| Fileprint | ghcr.io/boschglobal/dse-fileprint | Debugging tool for agents to verify volume mapping in MCP task sessions |
<!-- markdownlint-enable MD060 -->



## Contribute

Please refer to the [CONTRIBUTING.md](./CONTRIBUTING.md) file.


## License

Dynamic Simulation Environment Agents is open-sourced under the Apache-2.0 license.

See the [LICENSE](LICENSE) and [NOTICE](./NOTICE) files for details.


### Third Party Licenses

[Third Party Licenses](licenses/)
