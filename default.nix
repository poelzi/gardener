{ buildGoModule, fetchFromGitHub, lib }:
let


in buildGoModule rec {
  pname = "pet";
  version = "0.3.4";

  src = fetchFromGitHub {
    owner = "knqyf263";
    repo = "pet";
    rev = "v${version}";
    sha256 = "0m2fzpqxk7hrbxsgqplkg7h2p7gv6s1miymv3gvw0cz039skag0s";
  };

  modSha256 = "1879j77k96684wi554rkjxydrj8g3hpp0kvxz03sd8dmwr3lh83j";

  meta = with lib; {
    description = "Simple command-line snippet manager, written in Go";
    homepage = https://github.com/knqyf263/pet;
    license = licenses.mit;
    maintainers = with maintainers; [ kalbasit ];
    platforms = platforms.linux ++ platforms.darwin;
  };
}


# { buildGoModule
# , nix-gitignore
# }:
# let

# in buildGoModule rec {
#   pname = "gardner";
#   version = "0.18.0";
#   src = nix-gitignore.gitignoreSource [] ./.;
#   goPackagePath = "github.com/gardner/gardner";
#   #modSha256 = "1gqn2vm3wrc3gml8mhkf11sap3hkyzhi90qwzw0x93wv6vmm4mcy";
# }


# { nixroot ? (import <nixpkgs> { }), defaultLv2Plugins ? false, lv2Plugins ? [ ]
# , releaseMode ? false, enableKeyfinder ? true, buildType ? "auto", cFlags ? [ ]
# , useClang ? true, useClazy ? false, buildFolder ? "cbuild" }:
# let
#   inherit (nixroot)
#     buildGoPackage;

# in buildGoPackage rec {
#     name = "gardner";
#     buildInputs = [ ];
#     goPackagePath = "github.com/gardener/gardener";
# }

# { pkgs ? import <nixpkgs> {} }:

# let
#   inherit (pkgs) stdenv buildEnv;

#   goPackages = buildEnv {
#     name = "go-packages";
#     paths =
#       [ # Dependencies...
#       ];
#   };
# in

# stdenv.mkDerivation rec {
#   name = "gardner-${version}";
#   version = "0.18.0";

#   src = ./.;

#   nativeBuildInputs = [ pkgs.go ];

#   buildPhase = ''
#     mkdir -p build/go/src/github.com/lnl7
#     ln -sfn ../../../../.. build/go/src/github.com/lnl7/foo
#     export GOPATH="${goPackages}:$PWD/build/go"
#     go build github.com/lnl7/foo
#   '';

#   installPhase = ''
#     mkdir -p $out/bin
#     cp foo $out/bin/foo
#   '';

#   shellHook = ''
#     cd ${builtins.toString ./.}
#     mkdir -p build/go/src/github.com/lnl7
#     ln -sfn ../../../../.. build/go/src/github.com/lnl7/foo
#     export GOPATH="${goPackages}:$PWD/build/go";
#   '';