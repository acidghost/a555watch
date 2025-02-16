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
      version = "SNAPSHOT";
    in {
      packages = {
        default = pkgs.buildGo123Module {
          inherit version;
          pname = "a555watch";
          src = ./.;
          vendorHash = null;
          env.CGO_ENABLED = 0;
          ldflags = [
            "-X main.buildVersion=${version}"
            "-X main.buildCommit=${self.rev or "dirty"}"
            "-X main.buildDate=${self.lastModifiedDate}"
          ];
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
