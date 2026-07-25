package sandbox

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
)

const (
	// volumePrefix namespaces the persistent Docker volume backing each sandbox.
	volumePrefix = "jardiniere-sb-"
	// branchPrefix namespaces the branch a fresh clone starts on.
	branchPrefix = "jard/"
)

// Name validates a given sandbox name or generates one.
func Name(requested string) (string, error) {
	if requested == "" {
		return randomName(), nil
	}
	return sanitizeName(requested)
}

// volumeName is the persistent Docker volume backing the named sandbox's clone.
func volumeName(name string) string { return volumePrefix + name }

// startBranch is the branch a fresh clone of the named sandbox starts on.
func startBranch(name string) string { return branchPrefix + name }

// nameRE constrains a sandbox name to characters valid in a container hostname.
var nameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

// sanitizeName validates a user-supplied sandbox name.
func sanitizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !nameRE.MatchString(name) {
		return "", fmt.Errorf("invalid sandbox name %q: start with a letter or digit and use only letters, digits, and '-'", name)
	}
	return name, nil
}

// pickName builds an "adjective-plant" name.
func pickName(pick func(n int) int) string {
	return adjectives[pick(len(adjectives))] + "-" + plants[pick(len(plants))]
}

// randomName generates a display name like "verdant-fern".
func randomName() string { return pickName(rand.IntN) }

// word lists for generated sandbox names.
var (
	adjectives = []string{
		"verdant", "leafy", "blooming", "budding", "mossy", "ferny", "thorny",
		"dewy", "lush", "fragrant", "hardy", "creeping", "climbing", "trailing",
		"potted", "sprouting", "flowering", "evergreen", "tender", "shady",
		"sunlit", "wild", "grafted", "rooted",
	}
	plants = []string{
		"fern", "ivy", "moss", "thyme", "sage", "basil", "clover", "willow",
		"cedar", "birch", "hazel", "laurel", "myrtle", "violet", "poppy", "aster",
		"dahlia", "zinnia", "lupine", "sorrel", "fennel", "nettle", "heather", "jasmine",
	}
)
