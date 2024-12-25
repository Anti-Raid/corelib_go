// From silverpelt/canonical_module
package silverpelt

type CanonicalCommandArgument struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Required    bool     `json:"required"`
	Choices     []string `json:"choices"`
}

type CanonicalCommand struct {
	Name               string                     `json:"name"`
	QualifiedName      string                     `json:"qualified_name"`
	Description        *string                    `json:"description"`
	NSFW               bool                       `json:"nsfw"`
	Subcommands        []CanonicalCommand         `json:"subcommands"`
	SubcommandRequired bool                       `json:"subcommand_required"`
	Arguments          []CanonicalCommandArgument `json:"arguments"`
}
