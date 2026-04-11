package prompts

import (
	"embed"
	"fmt"
)

//go:embed *.tmpl
var defaultTemplates embed.FS

// Read 返回内嵌的默认 prompt 文本。
func Read(name string) (string, error) {
	data, err := defaultTemplates.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read embedded prompt %s: %w", name, err)
	}

	return string(data), nil
}
