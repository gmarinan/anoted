package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// marshalPreservingComments renders cfg as YAML, keeping the comments, key
// order and formatting of the file already at path.
//
// Save marshalled the struct straight to YAML, so every write from the Config
// tab — including incidental ones, like picking a file manager — silently
// destroyed every comment the user had written in their config, along with the
// explanatory header anoted itself ships. The struct is still the source of
// truth for values; the existing file is the source of truth for presentation.
//
// Any failure falls back to a plain marshal, because writing a correct config
// without comments is much better than not writing one at all.
func marshalPreservingComments(path string, cfg Config) []byte {
	fresh, err := yaml.Marshal(&cfg)
	if err != nil {
		return nil
	}

	old, err := os.ReadFile(path)
	if err != nil {
		return fresh
	}

	var oldDoc, newDoc yaml.Node
	if err := yaml.Unmarshal(old, &oldDoc); err != nil {
		return fresh
	}
	if err := yaml.Unmarshal(fresh, &newDoc); err != nil {
		return fresh
	}
	if oldDoc.Kind != yaml.DocumentNode || newDoc.Kind != yaml.DocumentNode ||
		len(oldDoc.Content) == 0 || len(newDoc.Content) == 0 {
		return fresh
	}

	merged := mergeNode(oldDoc.Content[0], newDoc.Content[0])
	out := &yaml.Node{
		Kind:        yaml.DocumentNode,
		Content:     []*yaml.Node{merged},
		HeadComment: oldDoc.HeadComment,
		FootComment: oldDoc.FootComment,
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		return fresh
	}
	return data
}

// mergeNode returns fresh, carrying over any comments the corresponding node in
// old had. Values always come from fresh; only presentation comes from old.
func mergeNode(old, fresh *yaml.Node) *yaml.Node {
	if old == nil || fresh == nil {
		return fresh
	}
	if old.Kind == yaml.MappingNode && fresh.Kind == yaml.MappingNode {
		return mergeMapping(old, fresh)
	}
	carryComments(old, fresh)
	return fresh
}

func mergeMapping(old, fresh *yaml.Node) *yaml.Node {
	carryComments(old, fresh)

	// YAML mappings store keys and values as alternating Content entries.
	oldByKey := make(map[string]int, len(old.Content)/2)
	for i := 0; i+1 < len(old.Content); i += 2 {
		oldByKey[old.Content[i].Value] = i
	}

	for i := 0; i+1 < len(fresh.Content); i += 2 {
		freshKey, freshVal := fresh.Content[i], fresh.Content[i+1]
		j, ok := oldByKey[freshKey.Value]
		if !ok {
			// A key the user's file does not have yet — a new setting from an
			// upgrade. Keep it, with no comments to inherit.
			continue
		}
		oldKey, oldVal := old.Content[j], old.Content[j+1]
		carryComments(oldKey, freshKey)
		fresh.Content[i+1] = mergeNode(oldVal, freshVal)
	}
	return fresh
}

// carryComments copies the comments attached to src onto dst, without
// overwriting comments the fresh tree already has (it normally has none).
func carryComments(src, dst *yaml.Node) {
	if src == nil || dst == nil {
		return
	}
	if dst.HeadComment == "" {
		dst.HeadComment = src.HeadComment
	}
	if dst.LineComment == "" {
		dst.LineComment = src.LineComment
	}
	if dst.FootComment == "" {
		dst.FootComment = src.FootComment
	}
}
