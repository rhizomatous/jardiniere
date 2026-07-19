{
  # dev environment for jardinière
  description = "jardinière dev environment";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      forAllSystems = f:
        nixpkgs.lib.genAttrs [
          "x86_64-linux"
          "aarch64-linux"
          "x86_64-darwin"
          "aarch64-darwin"
        ] (system: f nixpkgs.legacyPackages.${system});
    in {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            git
            gnumake
            golangci-lint # lint & static analysis
            gofumpt # stricter gofmt
            lefthook # git hooks manager
            goreleaser # release build & publish
          ];
          shellHook = ''
            # install the git hooks defined in lefthook.yml
            lefthook install >/dev/null 2>&1 || true
            echo "🪴 jardinière dev shell (go $(go version | cut -d' ' -f3))"
          '';
        };
      });
    };
}
