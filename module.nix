{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.programs.agent-usage-check;
  yamlFormat = pkgs.formats.yaml { };
  defaultPackage = pkgs.callPackage ./default.nix { };
in
{
  options.programs.agent-usage-check = {
    enable = lib.mkEnableOption "agent-usage-check tool and configuration";

    package = lib.mkOption {
      type = lib.types.package;
      default = defaultPackage;
      defaultText = lib.literalExpression "pkgs.callPackage ./default.nix { }";
      description = "The agent-usage-check package to install.";
    };

    settings = lib.mkOption {
      type = yamlFormat.type;
      default = { };
      example = lib.literalExpression ''
        {
          display = "remaining";
          interval = "15m";
          remaining_char = "■";
          used_char = "□";
          icon = "󰫣";
        }
      '';
      description = "Configuration written to ~/.config/agent-usage-check/config.yaml";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    xdg.configFile."agent-usage-check/config.yaml" = lib.mkIf (cfg.settings != { }) {
      source = yamlFormat.generate "agent-usage-check-config.yaml" cfg.settings;
    };
  };
}
