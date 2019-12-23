package lm

import "fmt"

type LMClientError struct {
	prefix       string
	ResponseBody string
	StatusCode   int
}

func (e *LMClientError) Error() string {
	fullPrefix := ""
	if e.prefix != "" {
		fullPrefix = fmt.Sprintf("%s -> ", e.prefix)
	}
	return fmt.Sprintf("%sStatusCode: %d, Body: %s", fullPrefix, e.StatusCode, e.ResponseBody)
}
