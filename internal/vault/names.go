package vault

import (
	"fmt"
	"regexp"
)

// validVaultName matches project/agent key rules: safe as a yq path segment.
var validVaultName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,30}$`)

// ValidateVaultName rejects names that could alter yq path semantics in DecryptForAgent.
func ValidateVaultName(name string) error {
	if !validVaultName.MatchString(name) {
		return fmt.Errorf("vault name %q must match %s", name, validVaultName.String())
	}
	return nil
}
