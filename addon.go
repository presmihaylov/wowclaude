// Package wowclaude embeds the addon so the daemon binary can install it into the game folder.
package wowclaude

import "embed"

//go:embed addon/WoWClaude/*.toc addon/WoWClaude/*.lua
var Addon embed.FS
