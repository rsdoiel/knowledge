package knowledge

import (
	"fmt"
	"strings"

	"github.com/rsdoiel/fountain"
	"gopkg.in/yaml.v3"
)

/** StoryMeta is the story-structure metadata a manuscript carries for one
 * scene: when it happens, where, whose point of view, how finished it is,
 * who is in it, and its links to other scenes (DR-0075). It is read from a
 * document's front matter, or, for a Fountain scene, from a one-line
 * boneyard after the heading (DR-0078). Every field is optional.
 *
 * Example:
 *   m := knowledge.StoryMeta{StoryTime: "1969-10-29", POV: "Anna"}
 *   fmt.Println(m.IsZero()) // false
 */
type StoryMeta struct {
	Event       string
	StoryTime   string
	Place       string
	POV         string
	Status      string
	Characters  []string
	Foreshadows []string
	PaysOff     []string
	Precedes    []string
	Learns      []string
	Knows       []string
}

/** IsZero reports whether no story field is set.
 *
 * Returns:
 *   bool — true when m carries no story metadata at all.
 *
 * Example:
 *   knowledge.StoryMeta{}.IsZero() // true
 */
func (m StoryMeta) IsZero() bool {
	return m.Event == "" && m.StoryTime == "" && m.Place == "" && m.POV == "" && m.Status == "" &&
		len(m.Characters) == 0 && len(m.Foreshadows) == 0 && len(m.PaysOff) == 0 && len(m.Precedes) == 0 &&
		len(m.Learns) == 0 && len(m.Knows) == 0
}

// isNarrative reports whether m carries a key that only a story would have.
// status and place are everyday front matter keys, so they alone do not make
// a file an event (DR-0079 item 4).
func (m StoryMeta) isNarrative() bool {
	return m.Event != "" || m.StoryTime != "" || m.POV != "" || len(m.Characters) > 0 ||
		len(m.Foreshadows) > 0 || len(m.PaysOff) > 0 || len(m.Precedes) > 0 ||
		len(m.Learns) > 0 || len(m.Knows) > 0
}

// yamlList decodes a YAML sequence, or a single scalar as a one-item list,
// so `characters: Anna` and `characters: [Anna, Bram]` both read.
type yamlList []string

func (l *yamlList) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.SequenceNode:
		var out []string
		if err := n.Decode(&out); err != nil {
			return err
		}
		*l = out
	case yaml.ScalarNode:
		if s := strings.TrimSpace(n.Value); s != "" {
			*l = []string{s}
		}
	default:
		return fmt.Errorf("line %d: want a list or a single value", n.Line)
	}
	return nil
}

// storyFrontmatter is the story slice of a document's YAML front matter.
// Kept apart from documentFrontmatter so the keys the generic reader drops
// are read here, once, without widening that struct's contract.
type storyFrontmatter struct {
	Event       string   `yaml:"event"`
	StoryTime   string   `yaml:"story_time"`
	Place       string   `yaml:"place"`
	POV         string   `yaml:"pov"`
	Status      string   `yaml:"status"`
	Characters  yamlList `yaml:"characters"`
	Foreshadows yamlList `yaml:"foreshadows"`
	PaysOff     yamlList `yaml:"pays_off"`
	Precedes    yamlList `yaml:"precedes"`
	Learns      yamlList `yaml:"learns"`
	Knows       yamlList `yaml:"knows"`
}

// extractStoryFrontmatter reads the story keys from a document's leading
// front matter block. No block, or a block extractFrontmatter already
// warned about, yields a zero StoryMeta and no warning: that is the normal
// case for a manuscript with no story metadata.
func extractStoryFrontmatter(data []byte) (StoryMeta, []string) {
	front, _, err := splitFrontmatter(string(data))
	if err != nil {
		return StoryMeta{}, nil
	}
	var fm storyFrontmatter
	if err := yaml.Unmarshal([]byte(front), &fm); err != nil {
		return StoryMeta{}, []string{fmt.Sprintf("story front matter: %v", err)}
	}
	learns, learnWarns := cleanKnowledgeItems("story front matter", "learns", fm.Learns)
	knows, knowWarns := cleanKnowledgeItems("story front matter", "knows", fm.Knows)
	return StoryMeta{
		Event:     strings.TrimSpace(fm.Event),
		StoryTime: strings.TrimSpace(fm.StoryTime), Place: strings.TrimSpace(fm.Place),
		POV: strings.TrimSpace(fm.POV), Status: strings.TrimSpace(fm.Status),
		Characters: fm.Characters, Foreshadows: fm.Foreshadows,
		PaysOff: fm.PaysOff, Precedes: fm.Precedes,
		Learns: learns, Knows: knows,
	}, append(learnWarns, knowWarns...)
}

/** ParseStoryBoneyard reads a Fountain boneyard element as story metadata
 * (DR-0078). The boneyard itself is delimited by the fountain module; this
 * reads only the key=value grammar inside it.
 *
 * Parameters:
 *   content (string) — a boneyard element's Content, delimiters included.
 *
 * Returns:
 *   StoryMeta — the metadata the boneyard sets.
 *   []string  — a message for each pair that was not understood (an
 *     unknown key, or a word with no "="); the rest of the pairs are kept.
 *   bool      — true when the boneyard is a story boneyard (its text starts
 *     "story:"); false for any other boneyard, which the caller leaves alone.
 *
 * Example:
 *   m, warn, ok := knowledge.ParseStoryBoneyard("/* story: pov=Anna; characters=A, B *" + "/")
 *   // ok, m.POV == "Anna", m.Characters == []string{"A", "B"}, warn == nil
 */
func ParseStoryBoneyard(content string) (StoryMeta, []string, bool) {
	text, found := strings.CutPrefix(fountain.BoneyardText(content), "story:")
	if !found {
		return StoryMeta{}, nil, false
	}
	meta, _, warnings := parseStoryPairs(text, "story boneyard", false)
	return meta, warnings, true
}

/** ParseStoryComment reads a Markdown HTML comment as the story metadata of
 * the scene above it (DR-0082): "<!-- story: id=12A; pov=Anna -->". It is
 * the Markdown counterpart of ParseStoryBoneyard, with the same key=value
 * grammar plus an id key, because a Markdown scene may take its id from the
 * comment as well as from a heading attribute.
 *
 * Parameters:
 *   content (string) — one HTML comment, delimiters included; it may span
 *     lines.
 *
 * Returns:
 *   StoryMeta — the metadata the comment sets.
 *   string    — the scene id the comment gives, "" when it has none.
 *   []string  — a message for each pair that was not understood (an unknown
 *     key, or a word with no "="); the rest of the pairs are kept.
 *   bool      — true when the comment is a story comment (its text starts
 *     "story:"); false for any other comment, which the caller leaves alone.
 *
 * Example:
 *   m, id, warn, ok := knowledge.ParseStoryComment("<!-- story: id=12A; pov=Anna -->")
 *   // ok, id == "12A", m.POV == "Anna", warn == nil
 */
func ParseStoryComment(content string) (StoryMeta, string, []string, bool) {
	inner, ok := strings.CutPrefix(strings.TrimSpace(content), "<!--")
	if !ok {
		return StoryMeta{}, "", nil, false
	}
	inner, ok = strings.CutSuffix(inner, "-->")
	if !ok {
		return StoryMeta{}, "", nil, false
	}
	text, found := strings.CutPrefix(strings.TrimSpace(inner), "story:")
	if !found {
		return StoryMeta{}, "", nil, false
	}
	meta, id, warnings := parseStoryPairs(text, "story comment", true)
	return meta, id, warnings, true
}

// parseStoryPairs reads the key=value pairs, separated by ";", that follow
// "story:" in a Fountain boneyard or a Markdown comment. label names the
// syntax in warnings. The id key is read only when allowID is set (the
// Markdown comment); in a boneyard the scene number is the identity, so id
// is an unknown key there.
func parseStoryPairs(text, label string, allowID bool) (meta StoryMeta, id string, warnings []string) {
	for _, pair := range strings.Split(text, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, val, found := strings.Cut(pair, "=")
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if !found {
			warnings = append(warnings, fmt.Sprintf("%s: %q has no \"=\"", label, pair))
			continue
		}
		switch key {
		case "id":
			if !allowID {
				warnings = append(warnings, fmt.Sprintf("%s: unknown key %q", label, key))
				continue
			}
			id = val
		case "event":
			meta.Event = val
		case "story_time":
			meta.StoryTime = val
		case "place":
			meta.Place = val
		case "pov":
			meta.POV = val
		case "status":
			meta.Status = val
		case "characters":
			meta.Characters = splitStoryList(val)
		case "foreshadows":
			meta.Foreshadows = splitStoryList(val)
		case "pays_off":
			meta.PaysOff = splitStoryList(val)
		case "precedes":
			meta.Precedes = splitStoryList(val)
		case "learns", "knows":
			// These two keys repeat, once per fact, because a fact may hold
			// a comma (DR-0083 item 3).
			kept, w := cleanKnowledgeItems(label, key, []string{val})
			warnings = append(warnings, w...)
			if key == "learns" {
				meta.Learns = append(meta.Learns, kept...)
			} else {
				meta.Knows = append(meta.Knows, kept...)
			}
		default:
			warnings = append(warnings, fmt.Sprintf("%s: unknown key %q", label, key))
		}
	}
	return meta, id, warnings
}

/** ParseKnowledgeItem splits one learns or knows item, "Character: fact", at its
 * first colon (DR-0083 item 1). The fact may itself contain colons.
 *
 * Parameters:
 *   item (string) — the item as written, e.g. "Anna: the letter is forged".
 *
 * Returns:
 *   string — the character, trimmed.
 *   string — the fact, trimmed, as written.
 *   bool   — false when there is no colon or either side is empty.
 *
 * Example:
 *   who, fact, ok := knowledge.ParseKnowledgeItem("Anna: the letter is forged")
 *   // who == "Anna", fact == "the letter is forged", ok
 */
func ParseKnowledgeItem(item string) (character, fact string, ok bool) {
	before, after, found := strings.Cut(item, ":")
	character, fact = strings.TrimSpace(before), strings.TrimSpace(after)
	return character, fact, found && character != "" && fact != ""
}

/** FactKey folds a fact's text to its identity (DR-0083 item 2): lower case, with
 * runs of white space collapsed to one space. No stemming.
 *
 * Parameters:
 *   fact (string) — the fact as written.
 *
 * Returns:
 *   string — the key two spellings of one fact share.
 *
 * Example:
 *   knowledge.FactKey("The  Letter is forged") // "the letter is forged"
 */
func FactKey(fact string) string {
	return strings.Join(strings.Fields(strings.ToLower(fact)), " ")
}

// cleanKnowledgeItems keeps the learns or knows items that read as
// "Character: fact" and warns, naming label and key, about each that does not.
func cleanKnowledgeItems(label, key string, items []string) (kept, warnings []string) {
	for _, item := range items {
		if item = strings.TrimSpace(item); item == "" {
			continue
		}
		if _, _, ok := ParseKnowledgeItem(item); !ok {
			warnings = append(warnings, fmt.Sprintf("%s: %s %q is not \"Character: fact\"", label, key, item))
			continue
		}
		kept = append(kept, item)
	}
	return kept, warnings
}

// splitStoryList splits a comma-separated boneyard value into trimmed,
// non-empty items.
func splitStoryList(val string) []string {
	var out []string
	for _, item := range strings.Split(val, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// The prefixes of the warnings a Markdown scene can raise while it is parsed
// (DR-0082 item 7). StoryCheck turns them into findings by prefix, so the
// messages are built from these constants and a test keeps the two in step.
const (
	warnNoID          = "story comment: no id"
	warnIDDisagrees   = "story comment: id"
	warnFountainStyle = "story heading:"
	warnCommentPrefix = "story comment:"
	warnFMIgnored     = "front matter "
)

// markdownWarningKind names the StoryCheck finding kind a Markdown parse
// warning belongs to, or "" for a warning StoryCheck does not report.
func markdownWarningKind(msg string) string {
	switch {
	case strings.HasPrefix(msg, warnNoID):
		return "scene-comment-no-id"
	case strings.HasPrefix(msg, warnIDDisagrees):
		return "scene-id-disagreement"
	case strings.HasPrefix(msg, warnFountainStyle):
		return "fountain-style-heading"
	case strings.HasPrefix(msg, warnCommentPrefix):
		return "scene-comment-syntax"
	case strings.HasPrefix(msg, warnFMIgnored) && strings.Contains(msg, " is ignored"):
		return "front-matter-ignored"
	}
	return ""
}
