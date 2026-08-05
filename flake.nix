{
  description = "A customizable CLI and status bar tool for checking AI agent rate limits & usage";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages = rec {
          default = agent-usage-check;
          agent-usage-check = pkgs.callPackage ./default.nix { };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            pkgs.gopls
          ];
        };
      }
    )
    // {
      homeManagerModules = rec {
        default = agent-usage-check;
        agent-usage-check = import ./module.nix;
      };

      homeManagerModule = self.homeManagerModules.default;
    };
}
