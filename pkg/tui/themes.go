package tui

import "sync"

type Theme struct {
	Name   string
	Colors map[string]string
}

var themes = map[string]Theme{
	"default": {
		Name: "default",
		Colors: map[string]string{
			"primary":   "#8daea5",
			"secondary": "#8BE9FD",
			"success":   "#50FA7B",
			"warning":   "#F1FA8C",
			"error":     "#FF5555",
			"dim":       "#6272A4",
			"text":      "#F8F8F2",
		},
	},
	"vintage_purple": {
		Name: "vintage_purple",
		Colors: map[string]string{
			"primary":   "#eaf0ce",
			"secondary": "#c0c5c1",
			"success":   "#7d8491",
			"warning":   "#574b60",
			"error":     "#3f334d",
			"dim":       "#545454",
			"text":      "#F8F8F2",
		},
	},
	"vintage_dark": {
		Name: "vintage_dark",
		Colors: map[string]string{
			"primary":   "#171614",
			"secondary": "#3a2618",
			"success":   "#754043",
			"warning":   "#9a8873",
			"error":     "#37423d",
			"dim":       "#3a2618",
			"text":      "#F8F8F2",
		},
	},
	"vintage_pinky": {
		Name: "vintage_pinky",
		Colors: map[string]string{
			"primary":   "#f7dad9",
			"secondary": "#443730",
			"success":   "#707852",
			"warning":   "#bc906a",
			"error":     "#f7dad9",
			"dim":       "#443730",
			"text":      "#F8F8F2",
		},
	},
	"blue_ish": {
		Name: "blue_ish",
		Colors: map[string]string{
			"primary":   "#2f6690",
			"secondary": "#3a7ca5",
			"success":   "#d9dcd6",
			"warning":   "#16425b",
			"error":     "#81c3d7",
			"dim":       "#3a7ca5",
			"text":      "#F8F8F2",
		},
	},
	"slate": {
		Name: "slate",
		Colors: map[string]string{
			"primary":   "#fcf7f8",
			"secondary": "#ced3dc",
			"success":   "#aba9c3",
			"warning":   "#275dad",
			"error":     "#5b616a",
			"dim":       "#5b616a",
			"text":      "#F8F8F2",
		},
	},
	"charcoal_tea": {
		Name: "charcoal_tea",
		Colors: map[string]string{
			"primary":   "#313628",
			"secondary": "#595358",
			"success":   "#857f74",
			"warning":   "#a4ac96",
			"error":     "#cadf9e",
			"dim":       "#595358",
			"text":      "#F8F8F2",
		},
	},
	"pastel_light": {
		Name: "pastel_light",
		Colors: map[string]string{
			"primary":   "#e8d7ff",
			"secondary": "#ffd3e8",
			"success":   "#ffd7d5",
			"warning":   "#f3ffe1",
			"error":     "#dfffd6",
			"dim":       "#ced3dc",
			"text":      "#F8F8F2",
		},
	},
	"sunflower_gold": {
		Name: "sunflower_gold",
		Colors: map[string]string{
			"primary":   "#ffc15e",
			"secondary": "#f7b05b",
			"success":   "#f7934c",
			"warning":   "#cc5803",
			"error":     "#1f1300",
			"dim":       "#f7b05b",
			"text":      "#F8F8F2",
		},
	},
	"muted_teal": {
		Name: "muted_teal",
		Colors: map[string]string{
			"primary":   "#8daa91",
			"secondary": "#788475",
			"success":   "#5e5d5c",
			"warning":   "#453643",
			"error":     "#28112b",
			"dim":       "#788475",
			"text":      "#F8F8F2",
		},
	},
}

var currentThemeName = "default"
var themeMu sync.RWMutex

// SetThemeName sets the active theme.
func SetThemeName(name string) {
	themeMu.Lock()
	if _, ok := themes[name]; ok {
		currentThemeName = name
	}
	themeMu.Unlock()
}

// GetThemeName returns the current theme name.
func GetThemeName() string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return currentThemeName
}

// GetTheme returns the current theme.
func GetTheme() *Theme {
	themeMu.RLock()
	defer themeMu.RUnlock()
	t := themes[currentThemeName]
	return &t
}

// TC returns a color hex string from the current theme by semantic key.
// Falls back to the key itself if not found (treats key as raw hex).
func TC(key string) string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	if t, ok := themes[currentThemeName]; ok {
		if c, ok := t.Colors[key]; ok {
			return c
		}
	}
	return key
}

func AvailableThemeNames() []string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	names := make([]string, 0, len(themes))
	for n := range themes {
		names = append(names, n)
	}
	return names
}
