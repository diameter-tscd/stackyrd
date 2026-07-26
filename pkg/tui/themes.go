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
			"accent":    "#FF79C6",
			"accent2":   "#BD93F9",
			"accent3":   "#FFB86C",
			"level_debug":   "#b3ebf8ff",
			"level_info":    "#9af8b1ff",
			"level_warn":    "#f5fac0ff",
			"level_error":   "#f67373ff",
			"level_fatal":   "#f82626ff",
			"pastel_good":   "#b0ffc4ff",
			"pastel_warn":   "#ffdab3ff",
			"pastel_bad":    "#ffaeaeff",
			"divider":       "#191919",
		},
	},
	"vintage_purple": {
		Name: "vintage_purple",
		Colors: map[string]string{
			"primary":     "#eaf0ce",
			"secondary":   "#c0c5c1",
			"success":     "#7d8491",
			"warning":     "#574b60",
			"error":       "#3f334d",
			"dim":         "#545454",
			"text":        "#F8F8F2",
			"accent":      "#550c18",
			"accent2":     "#3f334d",
			"accent3":     "#574b60",
			"level_debug":   "#c0c5c1",
			"level_info":    "#7d8491",
			"level_warn":    "#574b60",
			"level_error":   "#3f334d",
			"level_fatal":   "#550c18",
			"pastel_good":   "#c0c5c1",
			"pastel_warn":   "#7d8491",
			"pastel_bad":    "#3f334d",
			"divider":       "#545454",
		},
	},
	"vintage_dark": {
		Name: "vintage_dark",
		Colors: map[string]string{
			"primary":     "#171614",
			"secondary":   "#3a2618",
			"success":     "#754043",
			"warning":     "#9a8873",
			"error":       "#37423d",
			"dim":         "#3a2618",
			"text":        "#F8F8F2",
			"accent":      "#754043",
			"accent2":     "#9a8873",
			"accent3":     "#37423d",
			"level_debug":   "#9a8873",
			"level_info":    "#754043",
			"level_warn":    "#9a8873",
			"level_error":   "#37423d",
			"level_fatal":   "#171614",
			"pastel_good":   "#754043",
			"pastel_warn":   "#9a8873",
			"pastel_bad":    "#37423d",
			"divider":       "#3a2618",
		},
	},
	"vintage_pinky": {
		Name: "vintage_pinky",
		Colors: map[string]string{
			"primary":     "#550c18",
			"secondary":   "#443730",
			"success":     "#786452",
			"warning":     "#a5907e",
			"error":       "#f7dad9",
			"dim":         "#443730",
			"text":        "#F8F8F2",
			"accent":      "#a5907e",
			"accent2":     "#786452",
			"accent3":     "#f7dad9",
			"level_debug":   "#a5907e",
			"level_info":    "#786452",
			"level_warn":    "#a5907e",
			"level_error":   "#f7dad9",
			"level_fatal":   "#550c18",
			"pastel_good":   "#786452",
			"pastel_warn":   "#a5907e",
			"pastel_bad":    "#f7dad9",
			"divider":       "#443730",
		},
	},
	"blue_ish": {
		Name: "blue_ish",
		Colors: map[string]string{
			"primary":     "#2f6690",
			"secondary":   "#3a7ca5",
			"success":     "#d9dcd6",
			"warning":     "#16425b",
			"error":       "#81c3d7",
			"dim":         "#3a7ca5",
			"text":        "#F8F8F2",
			"accent":      "#16425b",
			"accent2":     "#81c3d7",
			"accent3":     "#d9dcd6",
			"level_debug":   "#81c3d7",
			"level_info":    "#d9dcd6",
			"level_warn":    "#16425b",
			"level_error":   "#81c3d7",
			"level_fatal":   "#2f6690",
			"pastel_good":   "#d9dcd6",
			"pastel_warn":   "#16425b",
			"pastel_bad":    "#81c3d7",
			"divider":       "#3a7ca5",
		},
	},
	"slate": {
		Name: "slate",
		Colors: map[string]string{
			"primary":     "#fcf7f8",
			"secondary":   "#ced3dc",
			"success":     "#aba9c3",
			"warning":     "#275dad",
			"error":       "#5b616a",
			"dim":         "#5b616a",
			"text":        "#F8F8F2",
			"accent":      "#275dad",
			"accent2":     "#aba9c3",
			"accent3":     "#5b616a",
			"level_debug":   "#ced3dc",
			"level_info":    "#aba9c3",
			"level_warn":    "#275dad",
			"level_error":   "#5b616a",
			"level_fatal":   "#2f6690",
			"pastel_good":   "#ced3dc",
			"pastel_warn":   "#aba9c3",
			"pastel_bad":    "#5b616a",
			"divider":       "#5b616a",
		},
	},
	"charcoal_tea": {
		Name: "charcoal_tea",
		Colors: map[string]string{
			"primary":     "#313628",
			"secondary":   "#595358",
			"success":     "#857f74",
			"warning":     "#a4ac96",
			"error":       "#cadf9e",
			"dim":         "#595358",
			"text":        "#F8F8F2",
			"accent":      "#a4ac96",
			"accent2":     "#857f74",
			"accent3":     "#cadf9e",
			"level_debug":   "#a4ac96",
			"level_info":    "#857f74",
			"level_warn":    "#a4ac96",
			"level_error":   "#cadf9e",
			"level_fatal":   "#313628",
			"pastel_good":   "#857f74",
			"pastel_warn":   "#a4ac96",
			"pastel_bad":    "#cadf9e",
			"divider":       "#595358",
		},
	},
	"pastel_light": {
		Name: "pastel_light",
		Colors: map[string]string{
			"primary":     "#e8d7ff",
			"secondary":   "#ffd3e8",
			"success":     "#ffd7d5",
			"warning":     "#f3ffe1",
			"error":       "#dfffd6",
			"dim":         "#ced3dc",
			"text":        "#383838",
			"accent":      "#ffd3e8",
			"accent2":     "#f3ffe1",
			"accent3":     "#dfffd6",
			"level_debug":   "#e8d7ff",
			"level_info":    "#ffd7d5",
			"level_warn":    "#f3ffe1",
			"level_error":   "#dfffd6",
			"level_fatal":   "#ffd3e8",
			"pastel_good":   "#ffd7d5",
			"pastel_warn":   "#f3ffe1",
			"pastel_bad":    "#dfffd6",
			"divider":       "#ced3dc",
		},
	},
	"sunflower_gold": {
		Name: "sunflower_gold",
		Colors: map[string]string{
			"primary":     "#ffc15e",
			"secondary":   "#f7b05b",
			"success":     "#f7934c",
			"warning":     "#cc5803",
			"error":       "#1f1300",
			"dim":         "#f7b05b",
			"text":        "#F8F8F2",
			"accent":      "#cc5803",
			"accent2":     "#1f1300",
			"accent3":     "#f7934c",
			"level_debug":   "#f7b05b",
			"level_info":    "#f7934c",
			"level_warn":    "#cc5803",
			"level_error":   "#1f1300",
			"level_fatal":   "#000000",
			"pastel_good":   "#f7934c",
			"pastel_warn":   "#cc5803",
			"pastel_bad":    "#1f1300",
			"divider":       "#f7b05b",
		},
	},
	"muted_teal": {
		Name: "muted_teal",
		Colors: map[string]string{
			"primary":     "#8daa91",
			"secondary":   "#788475",
			"success":     "#5e5d5c",
			"warning":     "#453643",
			"error":       "#28112b",
			"dim":         "#788475",
			"text":        "#F8F8F2",
			"accent":      "#453643",
			"accent2":     "#5e5d5c",
			"accent3":     "#28112b",
			"level_debug":   "#788475",
			"level_info":    "#5e5d5c",
			"level_warn":    "#453643",
			"level_error":   "#28112b",
			"level_fatal":   "#000000",
			"pastel_good":   "#5e5d5c",
			"pastel_warn":   "#453643",
			"pastel_bad":    "#28112b",
			"divider":       "#788475",
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