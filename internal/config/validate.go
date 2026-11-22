package config

// ValidateOptions tune Validate.
type ValidateOptions struct {
	// SkipPathCheck does not check that source paths exist.
	SkipPathCheck bool
}

// Validate checks the settings. Implemented in T1.4.
func (c *Config) Validate(ValidateOptions) error { return nil }
