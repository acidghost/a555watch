let
  # nixpkgs-unstable as of 2025-02-15 at 19:14
  nixpkgs = fetchTarball "https://github.com/NixOS/nixpkgs/archive/1128e89fd5e11bb25aedbfc287733c6502202ea9.tar.gz";
  pkgs = import nixpkgs { config = {}; overlays = []; };
in

pkgs.mkShellNoCC {
  packages = with pkgs; [
    go
    gopls
    golangci-lint
    goreleaser
    just
  ];
}
