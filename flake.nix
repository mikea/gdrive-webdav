{
  description = "A Nix-flake-based Go development environment";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-24.11";
    nixpkgs-go.url = "github:NixOS/nixpkgs/de0fe301211c267807afd11b12613f5511ff7433"; # go 1.24.1
  };

  outputs =
    {
      self,
      flake-utils,
      nixpkgs,
      ...
    }@inputs:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        deps = [
          inputs.nixpkgs-go.legacyPackages.${system}.go_1_24
          pkgs.git
          pkgs.git-chglog
          pkgs.gopls
          pkgs.golangci-lint
          pkgs.go-tools
        ];
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = deps;
        };

        formatter = pkgs.nixpkgs-fmt;
      }
    );
}
