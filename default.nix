{ buildGoModule }:
buildGoModule {
  pname = "agent-usage-check";
  version = "1.0.0";
  src = ./.;
  vendorHash = null;
}
