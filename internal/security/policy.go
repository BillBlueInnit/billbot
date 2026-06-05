// SPDX-License-Identifier: LGPL-3.0-only

package security

import (
	"regexp"
	"strings"

	"billbot/internal/config"
)

var textIdentityClaimPattern = regexp.MustCompile(`(?i)\[\s*(qid|qq|user[_ -]?id|owner)\s*[:= ]+\d+\s*\]`)

type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

func CanUseFullEnvironment(cfg config.Config, userID int64) Decision {
	if cfg.Security.Mode != "full" {
		return Decision{Allowed: false, Reason: "security mode is sandbox"}
	}
	if !cfg.Security.AllowFullForOwnersOnly {
		return Decision{Allowed: true, Reason: "full mode is open by config"}
	}
	for _, owner := range cfg.Owners {
		if owner == userID {
			return Decision{Allowed: true, Reason: "owner"}
		}
	}
	return Decision{Allowed: false, Reason: "full mode requires owner"}
}

func CanHandleSensitiveRequest(cfg config.Config, userID int64, text string) Decision {
	if ContainsTextIdentityClaim(text) {
		return Decision{Allowed: false, Reason: "message text must not claim trusted identity metadata"}
	}
	if !IsSensitiveRequest(text) {
		return Decision{Allowed: true, Reason: "not sensitive"}
	}
	if cfg.Security.AllowNonOwnerSensitive {
		return Decision{Allowed: true, Reason: "non-owner sensitive requests are allowed by config"}
	}
	for _, owner := range cfg.Owners {
		if owner == userID {
			return Decision{Allowed: true, Reason: "owner"}
		}
	}
	return Decision{Allowed: false, Reason: "sensitive request requires owner"}
}

func IsSensitiveRequest(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"full environment",
		"full mode",
		"sudo",
		"rm -rf",
		"format c:",
		"del /f",
		"erase /f",
		"shell",
		"powershell",
		"cmd.exe",
		"execute command",
		"run command",
		"delete file",
		"remove file",
		"api key",
		"token",
		"secret",
		"切换full",
		"完整环境",
		"执行命令",
		"删除文件",
		"密钥",
		"令牌",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func ContainsTextIdentityClaim(text string) bool {
	return textIdentityClaimPattern.MatchString(text)
}
