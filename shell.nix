{
  pkgs ? import <nixpkgs> { },
}:
pkgs.mkShell {
  nativeBuildInputs = [
    pkgs.buildPackages.go_1_23
    pkgs.gopls
    pkgs.buildPackages.golangci-lint
    pkgs.git-chglog
  ];
}
