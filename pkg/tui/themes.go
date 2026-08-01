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
	"forest_green": {
		Name: "forest_green",
		Colors: map[string]string{
			"primary":   "#2e8b57",
			"secondary": "#3cb371",
			"success":   "#a2d5a8",
			"warning":   "#c1e1c1",
			"error":     "#ff6b6b",
			"dim":       "#1a5f3a",
			"text":      "#F8F8F2",
		},
	},
	"desert_sand": {
		Name: "desert_sand",
		Colors: map[string]string{
			"primary":   "#d2b48c",
			"secondary": "#c2a47b",
			"success":   "#e6dcc6",
			"warning":   "#f5e6c4",
			"error":     "#ff8c00",
			"dim":       "#b8860b",
			"text":      "#F8F8F2",
		},
	},
	"ocean_blue": {
		Name: "ocean_blue",
		Colors: map[string]string{
			"primary":   "#006994",
			"secondary": "#0088b0",
			"success":   "#a0c9e8",
			"warning":   "#66b2ff",
			"error":     "#ff5e5e",
			"dim":       "#00557e",
			"text":      "#F8F8F2",
		},
	},
	"sunset_orange": {
		Name: "sunset_orange",
		Colors: map[string]string{
			"primary":   "#ff7f50",
			"secondary": "#ff6b35",
			"success":   "#ff9e7d",
			"warning":   "#ffb86b",
			"error":     "#ff4500",
			"dim":       "#cc5500",
			"text":      "#F8F8F2",
		},
	},
	"lavender_haze": {
		Name: "lavender_haze",
		Colors: map[string]string{
			"primary":   "#c8a2c8",
			"secondary": "#d8b5d8",
			"success":   "#e6d2e6",
			"warning":   "#f0e6f0",
			"error":     "#d699c6",
			"dim":       "#9a669a",
			"text":      "#F8F8F2",
		},
	},
	"mint_fresh": {
		Name: "mint_fresh",
		Colors: map[string]string{
			"primary":   "#3eb489",
			"secondary": "#98ff98",
			"success":   "#c8ffc8",
			"warning":   "#e6fff5",
			"error":     "#ff9999",
			"dim":       "#2e8b6e",
			"text":      "#F8F8F2",
		},
	},
	"coral_reef": {
		Name: "coral_reef",
		Colors: map[string]string{
			"primary":   "#ff6f61",
			"secondary": "#ff9e6d",
			"success":   "#ffb380",
			"warning":   "#ffcc99",
			"error":     "#d7263d",
			"dim":       "#b3492e",
			"text":      "#F8F8F2",
		},
	},
	"amber_glow": {
		Name: "amber_glow",
		Colors: map[string]string{
			"primary":   "#ffbf00",
			"secondary": "#ffd700",
			"success":   "#ffff99",
			"warning":   "#ffea99",
			"error":     "#ff8c00",
			"dim":       "#cc9900",
			"text":      "#F8F8F2",
		},
	},
	"plum_purple": {
		Name: "plum_purple",
		Colors: map[string]string{
			"primary":   "#9370db",
			"secondary": "#a052a0",
			"success":   "#c7b5e1",
			"warning":   "#d8b5d8",
			"error":     "#800080",
			"dim":       "#5f005f",
			"text":      "#F8F8F2",
		},
	},
	"teal_dark": {
		Name: "teal_dark",
		Colors: map[string]string{
			"primary":   "#006064",
			"secondary": "#008080",
			"success":   "#66b3a3",
			"warning":   "#00b3a0",
			"error":     "#ff6600",
			"dim":       "#004d40",
			"text":      "#F8F8F2",
		},
	},
	"rose_pink": {
		Name: "rose_pink",
		Colors: map[string]string{
			"primary":   "#ff9aa2",
			"secondary": "#ffb6b9",
			"success":   "#ffd9dc",
			"warning":   "#ffb3b3",
			"error":     "#ff4d4d",
			"dim":       "#cc2e2e",
			"text":      "#F8F8F2",
		},
	},
	"golden_yellow": {
		Name: "golden_yellow",
		Colors: map[string]string{
			"primary":   "#ffd700",
			"secondary": "#ffec8b",
			"success":   "#fff8a0",
			"warning":   "#ffe680",
			"error":     "#ff8c00",
			"dim":       "#ccb300",
			"text":      "#F8F8F2",
		},
	},
	"midnight_black": {
		Name: "midnight_black",
		Colors: map[string]string{
			"primary":   "#111111",
			"secondary": "#222222",
			"success":   "#444444",
			"warning":   "#666666",
			"error":     "#ff4d4d",
			"dim":       "#000000",
			"text":      "#F8F8F2",
		},
	},
	"rosepine": {
		Name: "rosepine",
		Colors: map[string]string{
			"primary":   "#c4a7e7",
			"secondary": "#ebbcba",
			"success":   "#9ccfd8",
			"warning":   "#f6c177",
			"error":     "#eb6f92",
			"dim":       "#908caa",
			"text":      "#e0def4",
		},
	},
	"tokyonight": {
		Name: "tokyonight",
		Colors: map[string]string{
			"primary":   "#7aa2f7",
			"secondary": "#bb9af7",
			"success":   "#9ece6a",
			"warning":   "#e0af68",
			"error":     "#f7768e",
			"dim":       "#565f89",
			"text":      "#c0caf5",
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
