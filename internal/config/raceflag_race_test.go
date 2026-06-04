//go:build race

package config_test

// raceEnabled reports whether the test binary was built with the race
// detector (-race). The detector adds several-fold runtime overhead, so
// wall-clock budget assertions (SC-003) are skipped in race builds and
// validated only in the non-race test jobs.
const raceEnabled = true
