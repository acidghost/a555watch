{
  description = "A 555 `watch` replacement";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    utils,
    ...
  }:
    utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {inherit system;};
    in {
      packages = {
        default = pkgs.buildGo123Module {
          pname = "a555watch";
          version = "1.0.0";
          src = ./.;
          vendorHash = null;
        };
      };

      defaultPackage = self.packages.${system}.default;

      devShell = with pkgs;
        mkShell {
          buildInputs = [
            go_1_23
            gopls
            golangci-lint
            goreleaser
            just
            uv
          ];
        };
    });
}
