{
  description = "Elfeed Offline - Go server for elfeed RSS reader";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        packages.default = pkgs.buildGoModule {
          pname = "elfeed-offline";
          version = "0.1.0";
          src = ./go-server;
          vendorHash = null;  # 표준 라이브러리만 사용
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_21
            gopls
            mkcert
          ];
        };
      }
    );
}
