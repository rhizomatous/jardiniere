{
  # example flake so jardinière can sandbox itself end-to-end
  description = "jardinière dev environment";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      forAllSystems = f:
        nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" ] (system:
          f nixpkgs.legacyPackages.${system});
    in {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go git ];
          shellHook = ''
            echo "🪴  welcome to the jardinière sandbox — go $(go version | cut -d' ' -f3)"
          '';
        };
      });
    };
}
