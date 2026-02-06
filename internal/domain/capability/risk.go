package capability

import "fmt"

type RiskLevel int

const (
	RiskNone RiskLevel = iota
	RiskLow
	RiskMedium
	RiskHigh
	RiskCritical
)

type RiskReport struct {
	Level       RiskLevel
	RiskFactors []RiskFactor
}

type RiskFactor struct {
	Level       RiskLevel
	Description string
	Rule        string
}

// AnalyzeRisk evaluates the risk level of a GrantSet.
func AnalyzeRisk(grants GrantSet) RiskReport {
	report := RiskReport{
		Level: RiskNone,
	}

	// Helper to add a factor
	addFactor := func(level RiskLevel, desc, rule string) {
		if level > RiskNone {
			report.RiskFactors = append(report.RiskFactors, RiskFactor{
				Level:       level,
				Description: desc,
				Rule:        rule,
			})
			if level > report.Level {
				report.Level = level
			}
		}
	}

	// 1. Analyze Network
	if grants.Network != nil {
		for _, rule := range grants.Network.Rules {
			ruleStr := fmt.Sprintf("Network: %s:%s", rule.Hosts, rule.Ports)

			// Critical: Wildcard host
			isWildcardHost := false
			for _, h := range rule.Hosts {
				if h == "*" || h == "0.0.0.0" {
					isWildcardHost = true
					break
				}
			}

			if isWildcardHost {
				addFactor(RiskCritical, "Unrestricted network access", ruleStr)
			} else {
				addFactor(RiskMedium, "Outbound network access", ruleStr)
			}
		}
	}

	// 2. Analyze FS
	if grants.FS != nil {
		for _, rule := range grants.FS.Rules {
			if len(rule.Write) > 0 {
				ruleStr := fmt.Sprintf("FS Write: %v", rule.Write)
				addFactor(RiskHigh, "Filesystem write access", ruleStr)
			}
			if len(rule.Read) > 0 {
				ruleStr := fmt.Sprintf("FS Read: %v", rule.Read)
				addFactor(RiskMedium, "Filesystem read access", ruleStr)
			}
		}
	}

	// 3. Analyze Exec
	if grants.Exec != nil && len(grants.Exec.Commands) > 0 {
		ruleStr := fmt.Sprintf("Exec: %v", grants.Exec.Commands)
		addFactor(RiskCritical, "Arbitrary command execution", ruleStr)
	}

	// 4. Analyze Env
	if grants.Env != nil && len(grants.Env.Variables) > 0 {
		ruleStr := fmt.Sprintf("Env: %v", grants.Env.Variables)
		addFactor(RiskLow, "Environment variable access", ruleStr)
	}

	return report
}
