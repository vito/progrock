{
  description = "a system for streaming and visualizing progress";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      supportedSystems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];
    in
    flake-utils.lib.eachSystem supportedSystems (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      rec {
        packages = { };

        apps = { };

        devShells = (pkgs.lib.optionalAttrs pkgs.stdenv.isLinux {
          default = pkgs.mkShell {
            nativeBuildInputs = with pkgs; [
              bashInteractive
              protobuf
              protoc-gen-go
              protoc-gen-go-grpc
              gnumake
            ];
          };
        });
      });
}
