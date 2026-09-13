package main

import "testing"

func TestAdvertiseAddrRewritesLoopbackAndUnspecified(t *testing.T) {
	t.Parallel()
	cases := []struct {
		listen, lan, want string
	}{
		{"0.0.0.0:2420", "10.218.220.10", "10.218.220.10:2420"},
		{":2420", "10.218.220.10", "10.218.220.10:2420"},
		{"127.0.0.1:2420", "10.218.220.10", "10.218.220.10:2420"},
		{"localhost:2420", "10.218.220.10", "10.218.220.10:2420"},
		{"10.218.220.10:9000", "192.168.1.2", "10.218.220.10:9000"},
	}
	for _, tc := range cases {
		if got := advertiseAddr(tc.listen, tc.lan); got != tc.want {
			t.Errorf("advertiseAddr(%q, %q) = %q, want %q", tc.listen, tc.lan, got, tc.want)
		}
	}
}

func TestNeedsAdvertiseRewrite(t *testing.T) {
	t.Parallel()
	if !needsAdvertiseRewrite("127.0.0.1") {
		t.Fatal("127.0.0.1 should be rewritten")
	}
	if needsAdvertiseRewrite("10.0.0.5") {
		t.Fatal("LAN IP should be kept")
	}
}
