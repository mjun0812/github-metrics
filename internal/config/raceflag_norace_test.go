//go:build !race

package config_test

// raceEnabled reports whether the test binary was built with the race
// detector (-race). See raceflag_race_test.go for the rationale.
const raceEnabled = false
