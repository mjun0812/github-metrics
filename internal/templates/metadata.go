package templates

// TemplateMetadata carries the subset of
// assets/templates/<name>/metadata.yml the engine consumes: account-kind
// gating (CheckAccount) and output-format gating (CheckFormat / the
// empty-format default in engine.dispatch). Other metadata.yml keys are
// ignored at parse time.
type TemplateMetadata struct {
	Supports []string `yaml:"supports"`
	Formats  []string `yaml:"formats"`
}
