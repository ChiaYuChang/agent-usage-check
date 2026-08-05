# agent-usage-check

A token-optimized, highly customizable CLI and status bar integration tool written in Go for monitoring AI agent usage rate limits and reset tokens.

Supports OpenAI ChatGPT / Codex backend APIs out-of-the-box, with an architecture designed for future multi-agent support (e.g., Claude, Ollama).

## Features

- **Status Bar Integration**: Emits JSON formatted for Linux desktop status bars (Ashell, Waybar, etc.).
- **Desktop Notifications**: `--details` mode generates detailed ASCII progress bar reports for `notify-send`.
- **Instant Response & Smart Caching**: Automatically updates `~/.config/agent-usage-check/cache.json` on fetch and falls back to cached state when offline.
- **YAML Configuration**: Supports `~/.config/agent-usage-check/config.yaml`.
- **First-Class Nix & Home-Manager Support**: Built with Nix Flakes and exports ready-to-use Home-Manager modules.

## Installation & Configuration

### Home-Manager (Nix Flake)

Add `agent-usage-check` to your `flake.nix`:

```nix
{
  inputs = {
    agent-usage-check.url = "github:cychang/agent-usage-check"; # Or path:~/Projects/agent-usage-check
  };

  outputs = { home-manager, agent-usage-check, ... }: {
    homeConfigurations."user" = home-manager.lib.homeManagerConfiguration {
      modules = [
        agent-usage-check.homeManagerModules.default
        {
          programs.agent-usage-check = {
            enable = true;
            settings = {
              display = "remaining";
              interval = "15m";
              remaining_char = "■";
              used_char = "□";
              icon = "󰫣";
            };
          };
        }
      ];
    };
  };
}
```

### Manual Configuration (`~/.config/agent-usage-check/config.yaml`)

```yaml
display: remaining # remaining or used
interval: 15m
remaining_char: "■"
used_char: "□"
icon: "󰫣"
json: false # set true to output structured JSON
watch: false # set true to continuously stream status updates
cache_path: "~/.config/agent-usage-check/cache.json"
auth_path: "~/.codex/auth.json"
```

## CLI Usage

```bash
# Print status bar JSON once and exit (default behavior)
agent-usage-check

# Continuously watch and stream status updates (for status bars / daemons)
agent-usage-check --watch

# Print structured agent JSON output once
agent-usage-check --json
# Output: {"name":"openai","icon":"󰫣","usage":{"5h":3,"1w":"N/A"},"alt":"available"}

# Print detailed usage report (fetches fresh data and updates cache)
agent-usage-check --details

# Override options via CLI flags
agent-usage-check --display used --interval 5m --watch
```

## Acknowledgements

Inspired by [doggy8088/codex-reset-checker](https://github.com/doggy8088/codex-reset-checker).

## License

MIT
