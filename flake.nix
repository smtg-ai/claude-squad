{
  description = "Claude Squad - Manage multiple AI agents like Claude Code, Aider, Codex, and Amp";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.buildGoModule {
            pname = "claude-squad";
            version = "1.0.17";

            src = ./.;

            vendorHash = "sha256-Rc0pIwnA0k99IKTvYkHV54RxtY87zY1TmmmMl+hYk6Q=";

            nativeBuildInputs = [ pkgs.git ];

            # Tests require filesystem access and tmux which aren't available in the sandbox
            doCheck = false;

            meta = with pkgs.lib; {
              description = "Manage multiple AI agents like Claude Code, Aider, Codex, and Amp";
              homepage = "https://github.com/smtg-ai/claude-squad";
              license = licenses.mit;
              mainProgram = "claude-squad";
            };
          };
        });

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/claude-squad";
        };
      });

      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go
              gopls
              gotools
              go-tools
            ];
          };
        });
    };
}
