{
  description = "envref — CLI tool for separating config from secrets in .env files";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = if (self ? shortRev) then self.shortRev else "dev";
      in
      {
        packages = {
          envref = pkgs.buildGoModule {
            pname = "envref";
            inherit version;
            src = ./.;
            vendorHash = null;

            subPackages = [ "cmd/envref" ];

            CGO_ENABLED = 0;

            ldflags = [
              "-s"
              "-w"
              "-X github.com/xcke/envref/internal/cmd.version=${version}"
            ];

            meta = with pkgs.lib; {
              description = "CLI tool for separating config from secrets in .env files";
              homepage = "https://github.com/xcke/envref";
              license = licenses.mit;
              maintainers = [ ];
              mainProgram = "envref";
            };
          };

          default = self.packages.${system}.envref;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            golangci-lint
            goreleaser
          ];
        };
      }
    );
}
