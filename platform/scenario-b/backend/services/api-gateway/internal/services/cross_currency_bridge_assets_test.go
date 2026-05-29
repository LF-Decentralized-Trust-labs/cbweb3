package services

import "testing"

func TestCrossCurrencyBridgeAssetsFromHub(t *testing.T) {
	cfg := &HubLiquidityConfig{
		SovereignHubTokenAAddress: "0x75c35c980c0d37ef46df04d31a140b65503c0eed",
		SovereignHubTokenBAddress: "0x82d50ad3c1091866e258fd0f1a7cc9674609d254",
	}
	assets := CrossCurrencyBridgeAssetsFromHub(cfg, "BRL", "ARS", "0xfiat", "spoke-a")
	if assets == nil {
		t.Fatal("expected assets")
	}
	if assets.WrappedSourceToken != cfg.SovereignHubTokenAAddress {
		t.Fatalf("wrapped source: got %s", assets.WrappedSourceToken)
	}
	if assets.WrappedTargetToken != cfg.SovereignHubTokenBAddress {
		t.Fatalf("wrapped target: got %s", assets.WrappedTargetToken)
	}
	if assets.NativeSourceToken != "0xfiat" {
		t.Fatalf("native: got %s", assets.NativeSourceToken)
	}
}
