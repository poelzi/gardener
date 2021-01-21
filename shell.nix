let
  sources = import ./nix/sources.nix;
  pkgs = import sources.nixpkgs {};


  # ginko = pkgs.buildGoModule rec {
  #     pname = "ginkgo";
  #     version = "1.14.2";

  #     vendorSha256 = null;
  #     src = pkgs.fetchFromGitHub {
  #         owner = "onsi";
  #         repo = "ginkgo";
  #         rev = "v${version}";
  #         sha256 = "1pvslbfrpzc8n99x33gyvk9aaz6lvdyyg6cj3axjzkyjxmh6d5kc";
  #     };
  # };

in
	# @go install -mod=vendor github.com/onsi/ginkgo/ginkgo
	# @go install -mod=vendor github.com/ahmetb/gen-crd-api-reference-docs
	# @go install -mod=vendor github.com/golang/mock/mockgen
	# @go install -mod=vendor sigs.k8s.io/controller-tools/cmd/controller-gen
	# @GO111MODULE=off go get github.com/prometheus/prometheus/cmd/promtool


# let
#   pkgs = import (import ./nix/sources.nix) {};
# in

{
  buildGoModule ? pkgs.buildGoModule
, nix-gitignore ? pkgs.nix-gitignore
, version ? "dev"
, docker ? pkgs.docker
, go ? pkgs.go_1_15
, installShellFiles ? pkgs.installShellFiles
, lib ? pkgs.lib
}:

buildGoModule rec {
  inherit version;

  pname = "gardner";

  src = nix-gitignore.gitignoreSource [ ".git" ".golangci.yaml" ".gitignore" ] ./.;

  vendorSha256 = null;

  #buildFlagsArray = [ "-ldflags=" "-X=github.com/kalbasit/swm/cmd.version=${version}" ];

  nativeBuildInputs = [ 
      go 
      # ginko
      ] ++
      (with pkgs; [
          protobuf
          goimports
          git
          unzip
          kubectl
          helm
          openvpn
          iproute
          minikube
          jq
          yaml2json
          gnumake
          kops
          tmux
          awscli2
      ]);

#   postInstall = ''
#     for shell in bash zsh fish; do
#       $out/bin/swm auto-complete $shell > swm.$shell
#       installShellCompletion swm.$shell
#     done
#     $out/bin/swm gen-doc man --path ./man
#     installManPage man/*.7
#     wrapProgram $out/bin/swm --prefix PATH : ${lib.makeBinPath [ fzf git tmux procps ]}
#   '';

#   doCheck = true;
#   preCheck = ''
#     export HOME=$NIX_BUILD_TOP/home
#     mkdir -p $HOME
#     git config --global user.email "nix-test@example.com"
#     git config --global user.name "Nix Test"
#   '';

  meta = with lib; {
    homepage = "https://gardner.cloud";
    description = "swm (Story-based Workflow Manager) is a Tmux session manager specifically designed for Story-based development workflow";
    license = licenses.mit;
    # maintainers = [ maintainers.kalbasit ];
  };
}

