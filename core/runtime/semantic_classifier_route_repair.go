package runtime

import "strings"

func repairClassifierOutputForAvailableTools(
	input ClassifierInput,
	output ClassifierOutput,
) ClassifierOutput {
	if output.Route != RouteAskUser {
		return output
	}
	if !hasAvailableBrowserTool(input.Tools) {
		return output
	}
	if promptRequiresPrivateAccess(input.UserPrompt) {
		return output
	}
	if !looksLikePublicBrowserTask(input.UserPrompt) {
		return output
	}
	return ClassifierOutput{Route: RouteExecuteTask, UserMessage: ""}
}

func hasAvailableBrowserTool(tools []ToolCatalogItem) bool {
	for _, tool := range tools {
		if !tool.Available {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(tool.Name))
		if strings.Contains(name, "browser") ||
			strings.Contains(name, "navigate") ||
			strings.Contains(name, "open") {
			return true
		}
	}
	return false
}

func looksLikePublicBrowserTask(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	if strings.Contains(text, "http://") || strings.Contains(text, "https://") {
		return true
	}
	if strings.Contains(text, ".ru") ||
		strings.Contains(text, ".com") ||
		strings.Contains(text, ".org") ||
		strings.Contains(text, ".net") ||
		strings.Contains(text, ".io") {
		return true
	}
	for _, marker := range []string{
		"open ",
		"browse ",
		"browser",
		"website",
		"web page",
		"page",
		"открой ",
		"открыть ",
		"сайт",
		"страниц",
		"заголов",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func promptRequiresPrivateAccess(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"login",
		"log in",
		"sign in",
		"account",
		"credentials",
		"password",
		"api key",
		"token",
		"private",
		"личный кабинет",
		"аккаунт",
		"пароль",
		"логин",
		"токен",
		"api ключ",
		"приват",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
