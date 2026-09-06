package bootstrap

import "github.com/spf13/cobra"

// redactSecret masks a secret for display, keeping a short prefix and suffix
// so the user can still recognize which value is configured.
func redactSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return "********"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

// maskFlagDefaults hides secret config values that were bound as flag defaults
// from --help output. Only the displayed default changes; parsed values and
// binder behavior are unaffected.
func maskFlagDefaults(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		if f := cmd.Flags().Lookup(name); f != nil {
			f.DefValue = redactSecret(f.DefValue)
		}
	}
}
