{ pkgs ? import <nixpkgs> {} }:
  pkgs.mkShell {
    nativeBuildInputs = [ 
      pkgs.buildPackages.go_1_23
      pkgs.buildPackages.golangci-lint 
    ];
}
