package dns

import (
	"context"
	"fmt"
	"net"
	"time"
)

func ResolveSrv(hostname string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, addrs, err := net.DefaultResolver.LookupSRV(ctx, "minecraft", "tcp", hostname)

	if err != nil {
		return "", fmt.Errorf("resolve srv %s failed: %v", hostname, err)
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("srv %s has empty result", hostname)
	}

	return fmt.Sprintf("%s:%d", addrs[0].Target, addrs[0].Port), nil
}
