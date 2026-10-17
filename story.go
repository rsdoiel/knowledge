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
	StoryTime   string
	Place       string
	POV         string
	Status      string
	Characters  []string
	Foreshadows []string
	PaysOff     []string
	Precedes    []string
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
	return m.StoryTime == "" && m.Place == "" && m.POV == "" && m.Status == "" &&
		len(m.Characters) == 0 && len(m.Foreshadows) == 0 && len(m.PaysOff) == 0 && len(m.Precedes) == 0
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
	StoryTime   string   `yaml:"story_time"`
	Place       string   `yaml:"place"`
	POV         string   `yaml:"pov"`
	Status      string   `yaml:"status"`
	Characters  yamlList `yaml:"characters"`
	Foreshadows yamlList `yaml:"foreshadows"`
	PaysOff     yamlList `yaml:"pays_off"`
	Precedes    yamlList `yaml:"precedes"`
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
	return StoryMeta{
		StoryTime: strings.TrimSpace(fm.StoryTime), Place: strings.TrimSpace(fm.Place),
		POV: strings.TrimSpace(fm.POV), Status: strings.TrimSpace(fm.Status),
		Characters: fm.Characters, Foreshadows: fm.Foreshadows,
		PaysOff: fm.PaysOff, Precedes: fm.Precedes,
	}, nil
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
	m := []string{"", text}
	var meta StoryMeta
	var warnings []string
	for _, pair := range strings.Split(m[1], ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, val, found := strings.Cut(pair, "=")
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if !found {
			warnings = append(warnings, fmt.Sprintf("story boneyard: %q has no \"=\"", pair))
			continue
		}
		switch key {
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
		default:
			warnings = append(warnings, fmt.Sprintf("story boneyard: unknown key %q", key))
		}
	}
	return meta, warnings, true
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
