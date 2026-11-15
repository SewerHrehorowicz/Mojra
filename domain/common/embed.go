package common

import "embed"

//go:embed templates/*.tpl ui_components/*.tpl styles/*.tss controllers/*.js icons/* fonts/*
var FS embed.FS
