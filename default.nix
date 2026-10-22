{ buildGoModule }:
let
  version = "1.0.1";
in
buildGoModule {
  pname = "agent-usage-check";
  inherit version;
  src = ./.;
  vendorHash = null;
  subPackages = [ "cmd" ];
  ldflags = [ "-X main.version=${version}" ];
  postInstall = ''
    mv $out/bin/cmd $out/bin/agent-usage-check
  '';
}
