/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
{
  description = "Nix flake for gardenctl-v2";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-23.11";
  };

  outputs = {
    self,
    nixpkgs,
    ...
  }: let
    pname = "gardenctl";

    # System types to support.
    supportedSystems = ["x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin"];

    # Helper function to generate an attrset '{ x86_64-linux = f "x86_64-linux"; ... }'.
    forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

    # Nixpkgs instantiated for supported system types.
    nixpkgsFor = forAllSystems (system: import nixpkgs {inherit system;});
  in {
    # Provide some binary packages for selected system types.
    packages = forAllSystems (system: let
      pkgs = nixpkgsFor.${system};
      inherit (pkgs) stdenv lib;
    in {
      ${pname} = pkgs.buildGo121Module rec {
        inherit pname self;
        version = lib.fileContents ./VERSION;
        splitVersion = lib.versions.splitVersion version;
        major = if ((lib.elemAt splitVersion 0) == "v") then 
          lib.elemAt splitVersion 1
        else 
          lib.elemAt splitVersion 0;
        minor = if ((lib.elemAt splitVersion 0) == "v") then
          lib.elemAt splitVersion 2
        else
          lib.elemAt splitVersion 1;
        gitCommit = if (self ? rev) then
          self.rev
        else
          self.dirtyRev;
        state = if (self ? rev) then
          "clean"
        else
          "dirty";

        # This vendorHash represents a dervative of all go.mod dependancies and needs to be adjusted with every change
        vendorHash = "sha256-YE6Xtm7OsY82AxnHuJFaV5tpOqMKftZr1Xiws+43uVA=";

        src = ./.;

        ldflags = [
          "-s"
          "-w"
          "-X k8s.io/component-base/version.gitMajor=${major}"
          "-X k8s.io/component-base/version.gitMinor=${minor}"
          "-X k8s.io/component-base/version.gitVersion=${version}"
          "-X k8s.io/component-base/version.gitTreeState=${state}"
          "-X k8s.io/component-base/version.gitCommit=${gitCommit}"
          "-X k8s.io/component-base/version/verflag.programName=${pname}"
          # "-X k8s.io/component-base/version.buildDate=1970-01-01T0:00:00+0000"
        ];

        CGO_ENABLED = 0;
        doCheck = false;
        subPackages = [
          "/"
        ];
        nativeBuildInputs = [pkgs.installShellFiles];

        postInstall = ''
          mv $out/bin/${pname}-v2 $out/bin/${pname}
          installShellCompletion --cmd ${pname} \
              --zsh  <($out/bin/${pname} completion zsh) \
              --bash <($out/bin/${pname} completion bash) \
              --fish <($out/bin/${pname} completion fish)
        '';

        meta = with lib; {
          description = "gardenctl facilitates the administration of one or many garden, seed and shoot clusters";
          longDescription = ''
            gardenctl is a command-line client for the Gardener.
            It facilitates the administration of one or many garden, seed and shoot clusters.
            Use this tool to configure access to clusters and configure cloud provider CLI tools.
            It also provides support for accessing cluster nodes via ssh.
          '';
          homepage = "https://github.com/gardener/gardenctl-v2";
          license = licenses.asl20;
          platforms = supportedSystems;
        };
      };
    });

    # Add dependencies that are only needed for development
    devShells = forAllSystems (system: let
      pkgs = nixpkgsFor.${system};
    in {
      default = pkgs.mkShell {
        buildInputs = with pkgs; [
          go_1_21   # golang 1.21
          gopls     # go language server
          gotools   # go imports
          go-tools  # static checks
          gnumake   # standard make
        ];
      };
    });

    # The default package for 'nix build'
    defaultPackage = forAllSystems (system: self.packages.${system}.${pname});
  };
}
