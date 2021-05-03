{ sources ? import ./nix/sources.nix }:
let
  pkgs = import sources.nixpkgs {};
in
let
  inherit (pkgs) lib buildGoModule fetchFromGitHub mkShell;

  spiff = buildGoModule rec {
    pname = "minio-exporter";
    version = "1.7.0-beta-1";
    rev = "v${version}";

    # goPackagePath = "github.com/mandelsoft/spiff";
    src = fetchFromGitHub {
      inherit rev;
      owner = "mandelsoft";
      repo = "spiff";
      sha256 = "09a7zd0crwqivgvgs63ywh06v834xhrj360xvm3nmcjrmg4ys3w3";
    };
    vendorSha256 = "1k0gy1xzh25bqxxh52mpshqshl6hlc9xivkgbr9spxz8nh6h4dl4";
    # goDeps = ./hack/nix/spiff/deps.nix;

    meta = with lib; {
      description = "In-domain YAML templating engine spiff++";
      homepage = "https://github.com/mandelsoft/spiffr";
      license = licenses.asl20;
      platforms = platforms.unix;
    };
  };

in pkgs.mkShell {
  nativeBuildInputs = with pkgs;
    [
      awscli
      coreutils
      curl
      docker
      git
      gnumake
      gnused
      go
      iproute
      kops
      kubectl
      kubernetes-helm
      minikube
      openvpn
      protobuf
      screen
      yaml2json
      parallel
    ] ++ [ spiff ];
}
