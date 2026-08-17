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
			"primary":   "#82aaff",
			"secondary": "#c099ff",
			"success":   "#c3e88d",
			"warning":   "#ff966c",
			"error":     "#ff757f",
			"dim":       "#636da6",
			"text":      "#c8d3f5",
		},
	},
	"opencode": {
		Name: "opencode",
		Colors: map[string]string{
			"primary":   "#fab283",
			"secondary": "#5c9cf5",
			"success":   "#7fd88f",
			"warning":   "#f5a742",
			"error":     "#e06c75",
			"dim":       "#6a6a6a",
			"text":      "#e0e0e0",
		},
	},
	"catppuccin": {
		Name: "catppuccin",
		Colors: map[string]string{
			"primary":   "#89b4fa",
			"secondary": "#cba6f7",
			"success":   "#a6e3a1",
			"warning":   "#fab387",
			"error":     "#f38ba8",
			"dim":       "#a6adc8",
			"text":      "#cdd6f4",
		},
	},
	"dracula": {
		Name: "dracula",
		Colors: map[string]string{
			"primary":   "#bd93f9",
			"secondary": "#ff79c6",
			"success":   "#50fa7b",
			"warning":   "#ffb86c",
			"error":     "#ff5555",
			"dim":       "#6272a4",
			"text":      "#f8f8f2",
		},
	},
	"flexoki": {
		Name: "flexoki",
		Colors: map[string]string{
			"primary":   "#4385BE",
			"secondary": "#8B7EC8",
			"success":   "#879A39",
			"warning":   "#D0A215",
			"error":     "#D14D41",
			"dim":       "#575653",
			"text":      "#B7B5AC",
		},
	},
	"gruvbox": {
		Name: "gruvbox",
		Colors: map[string]string{
			"primary":   "#83a598",
			"secondary": "#d3869b",
			"success":   "#b8bb26",
			"warning":   "#fabd2f",
			"error":     "#fb4934",
			"dim":       "#a89984",
			"text":      "#ebdbb2",
		},
	},
	"monokai": {
		Name: "monokai",
		Colors: map[string]string{
			"primary":   "#78dce8",
			"secondary": "#ab9df2",
			"success":   "#a9dc76",
			"warning":   "#fc9867",
			"error":     "#ff6188",
			"dim":       "#727072",
			"text":      "#fcfcfa",
		},
	},
	"onedark": {
		Name: "onedark",
		Colors: map[string]string{
			"primary":   "#61afef",
			"secondary": "#c678dd",
			"success":   "#98c379",
			"warning":   "#d19a66",
			"error":     "#e06c75",
			"dim":       "#5c6370",
			"text":      "#abb2bf",
		},
	},
	"tron": {
		Name: "tron",
		Colors: map[string]string{
			"primary":   "#00d9ff",
			"secondary": "#007fff",
			"success":   "#00ff8f",
			"warning":   "#ff9000",
			"error":     "#ff3333",
			"dim":       "#4d6b87",
			"text":      "#caf0ff",
		},
	},
	"amoled": {
		Name: "amoled",
		Colors: map[string]string{
			"primary":   "#b388ff",
			"secondary": "#ff4081",
			"success":   "#00ff88",
			"warning":   "#ffea00",
			"error":     "#ff1744",
			"dim":       "#000000",
			"text":      "#ffffff",
		},
	},
	"aura": {
		Name: "aura",
		Colors: map[string]string{
			"primary":   "#a277ff",
			"secondary": "#f694ff",
			"success":   "#61ffca",
			"warning":   "#ffca85",
			"error":     "#ff6767",
			"dim":       "#6d6d6d",
			"text":      "#edecee",
		},
	},
	"ayu": {
		Name: "ayu",
		Colors: map[string]string{
			"primary":   "#59C2FF",
			"secondary": "#D2A6FF",
			"success":   "#7FD962",
			"warning":   "#E6B673",
			"error":     "#D95757",
			"dim":       "#565B66",
			"text":      "#BFBDB6",
		},
	},
	"carbonfox": {
		Name: "carbonfox",
		Colors: map[string]string{
			"primary":   "#33b1ff",
			"secondary": "#78a9ff",
			"success":   "#25be6a",
			"warning":   "#f1c21b",
			"error":     "#ee5396",
			"dim":       "#7d848f",
			"text":      "#f2f4f8",
		},
	},
	"catppuccin-frappe": {
		Name: "catppuccin-frappe",
		Colors: map[string]string{
			"primary":   "#8da4e2",
			"secondary": "#ca9ee6",
			"success":   "#a6d189",
			"warning":   "#e5c890",
			"error":     "#e78284",
			"dim":       "#949cb8",
			"text":      "#c6d0f5",
		},
	},
	"catppuccin-macchiato": {
		Name: "catppuccin-macchiato",
		Colors: map[string]string{
			"primary":   "#8aadf4",
			"secondary": "#c6a0f6",
			"success":   "#a6da95",
			"warning":   "#eed49f",
			"error":     "#ed8796",
			"dim":       "#939ab7",
			"text":      "#cad3f5",
		},
	},
	"cobalt2": {
		Name: "cobalt2",
		Colors: map[string]string{
			"primary":   "#0088ff",
			"secondary": "#9a5feb",
			"success":   "#9eff80",
			"warning":   "#ffc600",
			"error":     "#ff0088",
			"dim":       "#adb7c9",
			"text":      "#ffffff",
		},
	},
	"cursor": {
		Name: "cursor",
		Colors: map[string]string{
			"primary":   "#88c0d0",
			"secondary": "#81a1c1",
			"success":   "#3fa266",
			"warning":   "#f1b467",
			"error":     "#e34671",
			"dim":       "#aaaaaa",
			"text":      "#e4e4e4",
		},
	},
	"everforest": {
		Name: "everforest",
		Colors: map[string]string{
			"primary":   "#a7c080",
			"secondary": "#7fbbb3",
			"success":   "#a7c080",
			"warning":   "#e69875",
			"error":     "#e67e80",
			"dim":       "#7a8478",
			"text":      "#d3c6aa",
		},
	},
	"github": {
		Name: "github",
		Colors: map[string]string{
			"primary":   "#58a6ff",
			"secondary": "#bc8cff",
			"success":   "#3fb950",
			"warning":   "#e3b341",
			"error":     "#f85149",
			"dim":       "#8b949e",
			"text":      "#c9d1d9",
		},
	},
	"kanagawa": {
		Name: "kanagawa",
		Colors: map[string]string{
			"primary":   "#7E9CD8",
			"secondary": "#957FB8",
			"success":   "#98BB6C",
			"warning":   "#D7A657",
			"error":     "#E82424",
			"dim":       "#727169",
			"text":      "#DCD7BA",
		},
	},
	"lucent-orng": {
		Name: "lucent-orng",
		Colors: map[string]string{
			"primary":   "#EC5B2B",
			"secondary": "#EE7948",
			"success":   "#6ba1e6",
			"warning":   "#EC5B2B",
			"error":     "#e06c75",
			"dim":       "#808080",
			"text":      "#eeeeee",
		},
	},
	"material": {
		Name: "material",
		Colors: map[string]string{
			"primary":   "#82aaff",
			"secondary": "#c792ea",
			"success":   "#c3e88d",
			"warning":   "#ffcb6b",
			"error":     "#f07178",
			"dim":       "#546e7a",
			"text":      "#eeffff",
		},
	},
	"matrix": {
		Name: "matrix",
		Colors: map[string]string{
			"primary":   "#2eff6a",
			"secondary": "#00efff",
			"success":   "#62ff94",
			"warning":   "#e6ff57",
			"error":     "#ff4b4b",
			"dim":       "#8ca391",
			"text":      "#62ff94",
		},
	},
	"mercury": {
		Name: "mercury",
		Colors: map[string]string{
			"primary":   "#8da4f5",
			"secondary": "#a7b6f8",
			"success":   "#77c599",
			"warning":   "#fc9b6f",
			"error":     "#fc92b4",
			"dim":       "#9d9da8",
			"text":      "#dddde5",
		},
	},
	"nightowl": {
		Name: "nightowl",
		Colors: map[string]string{
			"primary":   "#82AAFF",
			"secondary": "#7fdbca",
			"success":   "#c5e478",
			"warning":   "#ecc48d",
			"error":     "#EF5350",
			"dim":       "#5f7e97",
			"text":      "#d6deeb",
		},
	},
	"nord": {
		Name: "nord",
		Colors: map[string]string{
			"primary":   "#88C0D0",
			"secondary": "#81A1C1",
			"success":   "#A3BE8C",
			"warning":   "#D08770",
			"error":     "#BF616A",
			"dim":       "#8B95A7",
			"text":      "#ECEFF4",
		},
	},
	"oc-2": {
		Name: "oc-2",
		Colors: map[string]string{
			"primary":   "#fab283",
			"secondary": "#fab283",
			"success":   "#12c905",
			"warning":   "#fcd53a",
			"error":     "#fc533a",
			"dim":       "#1f1f1f",
			"text":      "#f1ece8",
		},
	},
	"one-dark": {
		Name: "one-dark",
		Colors: map[string]string{
			"primary":   "#61afef",
			"secondary": "#c678dd",
			"success":   "#98c379",
			"warning":   "#e5c07b",
			"error":     "#e06c75",
			"dim":       "#5c6370",
			"text":      "#abb2bf",
		},
	},
	"onedarkpro": {
		Name: "onedarkpro",
		Colors: map[string]string{
			"primary":   "#61afef",
			"secondary": "#e06c75",
			"success":   "#98c379",
			"warning":   "#e5c07b",
			"error":     "#e06c75",
			"dim":       "#1e222a",
			"text":      "#abb2bf",
		},
	},
	"orng": {
		Name: "orng",
		Colors: map[string]string{
			"primary":   "#EC5B2B",
			"secondary": "#EE7948",
			"success":   "#6ba1e6",
			"warning":   "#EC5B2B",
			"error":     "#e06c75",
			"dim":       "#808080",
			"text":      "#eeeeee",
		},
	},
	"osaka-jade": {
		Name: "osaka-jade",
		Colors: map[string]string{
			"primary":   "#2DD5B7",
			"secondary": "#D2689C",
			"success":   "#549e6a",
			"warning":   "#E5C736",
			"error":     "#FF5345",
			"dim":       "#53685B",
			"text":      "#C1C497",
		},
	},
	"palenight": {
		Name: "palenight",
		Colors: map[string]string{
			"primary":   "#82aaff",
			"secondary": "#c792ea",
			"success":   "#c3e88d",
			"warning":   "#ffcb6b",
			"error":     "#f07178",
			"dim":       "#676e95",
			"text":      "#a6accd",
		},
	},
	"shadesofpurple": {
		Name: "shadesofpurple",
		Colors: map[string]string{
			"primary":   "#c792ff",
			"secondary": "#ff7ac6",
			"success":   "#7be0b0",
			"warning":   "#ffd580",
			"error":     "#ff7ac6",
			"dim":       "#1a102b",
			"text":      "#f5f0ff",
		},
	},
	"solarized": {
		Name: "solarized",
		Colors: map[string]string{
			"primary":   "#268bd2",
			"secondary": "#6c71c4",
			"success":   "#859900",
			"warning":   "#b58900",
			"error":     "#dc322f",
			"dim":       "#586e75",
			"text":      "#839496",
		},
	},
	"synthwave84": {
		Name: "synthwave84",
		Colors: map[string]string{
			"primary":   "#36f9f6",
			"secondary": "#ff7edb",
			"success":   "#72f1b8",
			"warning":   "#fede5d",
			"error":     "#fe4450",
			"dim":       "#848bbd",
			"text":      "#ffffff",
		},
	},
	"vercel": {
		Name: "vercel",
		Colors: map[string]string{
			"primary":   "#0070F3",
			"secondary": "#52A8FF",
			"success":   "#46A758",
			"warning":   "#FFB224",
			"error":     "#E5484D",
			"dim":       "#878787",
			"text":      "#EDEDED",
		},
	},
	"zenburn": {
		Name: "zenburn",
		Colors: map[string]string{
			"primary":   "#8cd0d3",
			"secondary": "#dc8cc3",
			"success":   "#7f9f7f",
			"warning":   "#f0dfaf",
			"error":     "#cc9393",
			"dim":       "#9f9f9f",
			"text":      "#dcdccc",
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
	t := themes[currentThemeName]
	colors := make(map[string]string, len(t.Colors))
	for k, v := range t.Colors {
		colors[k] = v
	}
	themeMu.RUnlock()
	t.Colors = colors
	return &t
}

// ThemeExists reports whether a named theme is registered.
func ThemeExists(name string) bool {
	themeMu.RLock()
	defer themeMu.RUnlock()
	_, ok := themes[name]
	return ok
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
