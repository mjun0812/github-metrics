package main

// localeStrings collects every locale-varying literal the generator
// emits into a page: section headings, table column headers, the
// sample caption, and the References-list labels. The AUTOGEN marker
// text is intentionally NOT localized — the markers are structural
// anchors used by the extract/preserve regex and must stay stable
// across locales so re-runs of `make docs` are idempotent.
type localeStrings struct {
	// Code is the locale identifier used for internal branching
	// (`en`, `ja`).
	Code string
	// FileSuffix is appended to the slug when computing the output
	// path (English uses the canonical no-suffix form).
	FileSuffix string

	// Section headings (rendered as `## <Heading>` / `### <Heading>`).
	PluginHeading       string // used as `# {PluginHeading}: <slug>`
	SampleHeading       string
	WhenToUseHeading    string
	InputsHeading       string
	UsageHeading        string
	GHActionSubheading  string
	CLISubheading       string
	RequirementsHeading string
	NotesHeading        string
	ReferencesHeading   string

	// Prose emitted in the auto-generated Sample block.
	SampleCaption            string
	NoStandaloneSampleNotice string

	// Input-table column headers.
	InputColName     string
	InputColDesc     string
	InputColDefault  string
	InputColRequired string
	InputColType     string

	// Small labels inside the table body.
	YesLabel                 string
	NoLabel                  string
	NoDescriptionPlaceholder string
	NoInputsNotice           string

	// References-list prose.
	RefActionYml   string
	RefMetadataYml string
	RefSupports    string
	RefScopes      string

	// Generic fallback used only when `metadata.yml` supplies no
	// description AND the slug has no purpose-written override
	// (see defaultDescription).
	GenericPluginBlurb string
}

var enStrings = localeStrings{
	Code:                     "en",
	FileSuffix:               "",
	PluginHeading:            "Plugin",
	SampleHeading:            "Sample",
	WhenToUseHeading:         "When to use",
	InputsHeading:            "Configuration (inputs)",
	UsageHeading:             "Usage",
	GHActionSubheading:       "GitHub Action",
	CLISubheading:            "CLI",
	RequirementsHeading:      "Requirements",
	NotesHeading:             "Notes",
	ReferencesHeading:        "References",
	SampleCaption:            "Rendered with `--user mjun0812` data, with only this plugin enabled. Regenerate with `make docs-examples`.",
	NoStandaloneSampleNotice: "This plugin emits no standalone SVG; its inputs are documented below.",
	InputColName:             "Input",
	InputColDesc:             "Description",
	InputColDefault:          "Default",
	InputColRequired:         "Required",
	InputColType:             "Type",
	YesLabel:                 "yes",
	NoLabel:                  "no",
	NoDescriptionPlaceholder: "(no description)",
	NoInputsNotice:           "(This plugin has no dedicated inputs.)",
	RefActionYml:             "canonical input schema",
	RefMetadataYml:           "upstream metadata",
	RefSupports:              "Supported account types",
	RefScopes:                "Required scopes",
	GenericPluginBlurb:       "plugin output for GitHub metrics.",
}

var jaStrings = localeStrings{
	Code:                     "ja",
	FileSuffix:               "_ja",
	PluginHeading:            "プラグイン",
	SampleHeading:            "サンプル",
	WhenToUseHeading:         "利用シーン",
	InputsHeading:            "設定 (inputs)",
	UsageHeading:             "使い方",
	GHActionSubheading:       "GitHub Action",
	CLISubheading:            "CLI",
	RequirementsHeading:      "前提条件",
	NotesHeading:             "備考",
	ReferencesHeading:        "参考",
	SampleCaption:            "`--user mjun0812` のデータでこのプラグインのみを有効にしてレンダリングしたものです。`make docs-examples` で再生成できます。",
	NoStandaloneSampleNotice: "このプラグインは単独の SVG を出力しません。以下の入力設定を参照してください。",
	InputColName:             "入力",
	InputColDesc:             "説明",
	InputColDefault:          "既定値",
	InputColRequired:         "必須",
	InputColType:             "型",
	YesLabel:                 "yes",
	NoLabel:                  "no",
	NoDescriptionPlaceholder: "(説明なし)",
	NoInputsNotice:           "(このプラグインには専用の入力はありません。)",
	RefActionYml:             "入力スキーマのリファレンス",
	RefMetadataYml:           "upstream 由来の metadata",
	RefSupports:              "対応アカウント種別",
	RefScopes:                "必要なスコープ",
	GenericPluginBlurb:       "プラグインの出力です。",
}
