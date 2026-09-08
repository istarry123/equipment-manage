// Package webui 内嵌前端构建产物（由 web/ 目录 Vite 构建后输出到此 dist）。
package webui

import "embed"

// Dist 包含 Vite 构建后的全部静态资源（index.html 位于根）。
//
//go:embed all:dist
var Dist embed.FS
