{
  # dev environment & package for jardinière
  description = "jardinière: a Nix-based sandbox for running coding agents";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      # keep in sync with the latest release tag.
      version = "0.1.4";
      forAllSystems = f:
        nixpkgs.lib.genAttrs [
          "x86_64-linux"
          "aarch64-linux"
          "x86_64-darwin"
          "aarch64-darwin"
        ] (system: f nixpkgs.legacyPackages.${system});
      # build the jard binary against a given package set. shared by the
      # per-system packages and the overlay.
      mkJard = pkgs: pkgs.buildGoModule {
        pname = "jard";
        inherit version;
        src = ./.;
        vendorHash = "sha256-3KZxEYKy+D6gREwPeSEVv8pVW/IsfuQ1L9ENdsM24Bk=";
        subPackages = [ "cmd/jard" ];
        # inject the version into the same symbol the Makefile uses.
        ldflags = [ "-s" "-w" "-X" "main.version=${version}" ];
        meta = {
          description = "a Nix-based sandbox for running coding agents in isolated containers";
          homepage = "https://github.com/rhizomatous/jardiniere";
          license = pkgs.lib.licenses.mit;
          mainProgram = "jard";
        };
      };
    in {
      # `overlays.default` lets consumers get `pkgs.jard` after applying it.
      overlays.default = final: _prev: {
        jard = mkJard final;
      };

      packages = forAllSystems (pkgs: {
        default = self.packages.${pkgs.system}.jard;
        jard = mkJard pkgs;
      });

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
